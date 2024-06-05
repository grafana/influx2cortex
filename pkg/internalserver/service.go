package internalserver

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
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

	logger log.Logger

	config  ServiceConfig
	server  *http.Server
	errChan chan error
	ready   *atomic.Bool
}

func NewService(config ServiceConfig, logger log.Logger) (*Service, error) {
	if logger == nil {
		return nil, errors.New("logger should not be nil")
	}

	ready := &atomic.Bool{}
	ready.Store(true)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("not ready"))
		}
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
		ready:   ready,
	}
	s.Service = services.NewBasicService(s.start, s.run, s.stop).WithName("internal")

	return s, nil
}

// SetReady sets the response for the health check endpoint.
func (s *Service) SetReady(ready bool) {
	s.ready.Store(ready)
}

func (s *Service) start(_ context.Context) error {
	_ = level.Info(s.logger).Log("msg", "Starting internal http server", "addr", s.server.Addr)

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
		_ = level.Warn(s.logger).Log("msg", "shutting down internal http server due to failure", "failure", failureCase)
	} else {
		_ = level.Info(s.logger).Log("msg", "shutting down internal http server")
	}

	err := s.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("error during shutdown of internal http server: %w", err)
	}

	return nil
}
