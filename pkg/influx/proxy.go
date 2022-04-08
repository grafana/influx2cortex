package influx

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/grafana/influx2cortex/pkg/server/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

// ProxyConfig holds objects needed to start running an influx2cortex proxy
// server.
type ProxyConfig struct {
	// HTTPConfig is the configuration for the underlying http server. Usually
	// initialized by flag values via flagext.RegisterFlags.
	HTTPConfig server.Config
	// EnableAuth determines if the server will reject incoming requests that do
	// not have X-Scope-OrgID set.
	EnableAuth bool
	// RemoteWriteConfig is the configuration for the underlying http server. Usually
	// initialized by flag values via flagext.RegisterFlags.
	RemoteWriteConfig remotewrite.Config
	// Logger is the object that will do the logging for the server. If nil, will
	// use a LogfmtLogger on stdout.
	Logger log.Logger
	// Registerer registers metrics Collectors. If left nil, will use
	// prometheus.DefaultRegisterer.
	Registerer prometheus.Registerer
}

// ProxyService is the actual Influx Proxy dskit service.
type ProxyService struct {
	services.Service

	Logger log.Logger

	config ProxyConfig
	server *server.Server
}

// NewProxy creates a new remotewrite client
func NewProxy(conf ProxyConfig) (*ProxyService, error) {
	if conf.Registerer == nil {
		conf.Registerer = prometheus.DefaultRegisterer
	}
	remoteWriteRecorder := remotewrite.NewRecorder("influx_proxy", conf.Registerer)
	client, err := remotewrite.NewClient(conf.RemoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create remotewrite.API: %w", err)
	}

	return newProxyWithClient(conf, client)
}

// newProxyWithClient creates the influx API server with the given config options and
// the specified remotewrite client. It returns the HTTP server that is ready to Run.
func newProxyWithClient(conf ProxyConfig, client remotewrite.Client) (*ProxyService, error) {
	if conf.Logger == nil {
		conf.Logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	}

	recorder := NewRecorder(conf.Registerer)

	var authMiddleware middleware.Interface
	if conf.EnableAuth {
		authMiddleware = middleware.NewHTTPAuth(conf.Logger)
	} else {
		authMiddleware = middleware.HTTPFakeAuth{}
	}

	server, err := server.NewServer(conf.Logger, conf.HTTPConfig, mux.NewRouter(), []middleware.Interface{authMiddleware})
	if err != nil {
		return nil, fmt.Errorf("failed to create http server: %w", err)
	}

	api, err := NewAPI(conf.Logger, client, recorder)
	if err != nil {
		return nil, fmt.Errorf("failed to create influx API: %w", err)
	}

	api.Register(server.Router)
	err = recorder.RegisterVersionBuildTimestamp()
	if err != nil {
		return nil, fmt.Errorf("could not register version build timestamp: %w", err)
	}

	p := &ProxyService{
		Logger: conf.Logger,
		config: conf,
		server: server,
	}
	p.Service = services.NewBasicService(p.start, p.run, p.stop)
	return p, nil
}

// Addr returns the net.Addr for the configured server. This is useful in case
// it was started with port auto-selection so the port number can be retrieved.
func (p *ProxyService) Addr() net.Addr {
	return p.server.Addr()
}

func (p *ProxyService) start(_ context.Context) error {
	return nil
}

func (p *ProxyService) stop(_ error) error {
	p.server.Shutdown(nil)
	return nil
}

func (p *ProxyService) run(servCtx context.Context) error {
	errChan := make(chan error, 1)

	// the server does not listen for context canceling, so we have to start it
	// in a goroutine so we can listen for both.
	go func() {
		err := p.server.Run()
		errChan <- err
	}()

	for {
		select {
		case <-servCtx.Done():
			return servCtx.Err()
		case err := <-errChan:
			return err
		}
	}
}
