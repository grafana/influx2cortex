package internalserver

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	gokitlog "github.com/go-kit/log"
	"github.com/grafana/dskit/services"
	"github.com/grafana/influx2cortex/pkg/util/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	DefaultListenAddress           = "0.0.0.0"
	DefaultListenPort              = 8081
	DefaultGracefulShutdownTimeout = 5 * time.Second
)

type ServiceConfig struct {
	HTTPListenAddress       string
	HTTPListenPort          int
	GracefulShutdownTimeout time.Duration
}

func (c *ServiceConfig) RegisterFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.HTTPListenAddress, "internalserver.http-listen-address", DefaultListenAddress, "Sets the listen address for the internal http server")
	flags.IntVar(&c.HTTPListenPort, "internalserver.http-listen-port", DefaultListenPort, "Sets listen address port for the internal http server")
	flags.DurationVar(&c.GracefulShutdownTimeout, "internalserver.graceful-shutdown-timeout", DefaultGracefulShutdownTimeout, "Graceful shutdown period")
}

type Service struct {
	services.Service

	logger gokitlog.Logger

	config  ServiceConfig
	server  *http.Server
	errChan chan error
}

func NewService(config ServiceConfig, logger gokitlog.Logger) (*Service, error) {
	if logger == nil {
		return nil, errors.New("logger should not be nil")
	}

	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.HTTPListenAddress, config.HTTPListenPort),
		Handler: mux,
	}

	s := &Service{
		logger:  logger,
		config:  config,
		server:  httpServer,
		errChan: make(chan error, 1),
	}
	s.Service = services.NewBasicService(s.start, s.run, s.stop).WithName("internal")

	return s, nil
}

func (s *Service) start(_ context.Context) error {
	log.Info(s.logger, "msg", "Starting internal http server", "addr", s.server.Addr)

	go func() {
		err := s.server.ListenAndServe()
		s.errChan <- err
	}()

	return nil
}

func (s *Service) run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-s.errChan:
			return err
		}
	}
}

func (s *Service) stop(failureCase error) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.GracefulShutdownTimeout)
	defer cancel()

	if failureCase != nil && !errors.Is(failureCase, context.Canceled) {
		log.Warn(s.logger, "msg", "shutting down internal http server due to failure", "failure", failureCase)
	} else {
		log.Info(s.logger, "msg", "shutting down internal http server")
	}

	err := s.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error during shutdown of internal http server: %w", err)
	}

	return nil
}
