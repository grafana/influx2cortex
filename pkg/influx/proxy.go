package influx

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/grafana/mimir-graphite/v2/pkg/appcommon"
	"github.com/grafana/mimir-graphite/v2/pkg/remotewrite"
	"github.com/grafana/mimir-graphite/v2/pkg/server"
	"github.com/grafana/mimir-graphite/v2/pkg/server/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// Upped to 10MB based on empirical evidence of proxy receiving batches from telegraf agent
	DefaultMaxRequestSizeBytes = 10 << 20 // 10 MB
	serviceName                = "influx-write-proxy"
)

// ProxyConfig holds objects needed to start running an influx2cortex proxy
// server.
type ProxyConfig struct {
	// HTTPConfig is the configuration for the underlying http server. Usually
	// initialized by flag values via flagext.RegisterFlags.
	HTTPConfig server.Config
	// RemoteWriteConfig is the configuration for the underlying remote write client. Usually
	// initialized by flag values via flagext.RegisterFlags.
	RemoteWriteConfig remotewrite.Config
	// EnableAuth determines if the server will reject incoming requests that do
	// not have X-Scope-OrgID set.
	EnableAuth bool
	// Logger is the object that will do the logging for the server. If nil, will
	// use a LogfmtLogger on stdout.
	Logger log.Logger
	// Registerer registers metrics Collectors. If left nil, will use
	// prometheus.DefaultRegisterer.
	Registerer prometheus.Registerer
	// MaxRequestSizeBytes limits the size of an incoming request. Any value less than or equal to 0 means no limit.
	MaxRequestSizeBytes int
}

func (c *ProxyConfig) RegisterFlags(flags *flag.FlagSet) {
	c.HTTPConfig.RegisterFlags(flags)
	c.RemoteWriteConfig.RegisterFlags(flags)

	flags.BoolVar(&c.EnableAuth, "auth.enable", true, "require X-Scope-OrgId header")
	flags.IntVar(&c.MaxRequestSizeBytes, "max.request.size.bytes", DefaultMaxRequestSizeBytes, "limit the size of incoming batches; 0 for no limit")
}

// ProxyService is the actual Influx Proxy dskit service.
type ProxyService struct {
	services.Service

	logger log.Logger

	config  ProxyConfig
	server  *server.Server
	errChan chan error

	tracerCloser func() error
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
	router := mux.NewRouter()

	var authMiddleware middleware.Interface
	if conf.EnableAuth {
		authMiddleware = middleware.NewHTTPAuth(conf.Logger)
	} else {
		authMiddleware = middleware.HTTPFakeAuth{}
	}

	tracer, tracerCloser, err := appcommon.NewTracer(serviceName, conf.Logger)
	if err != nil {
		return nil, err
	}
	tracerMiddleware := middleware.NewTracer(router, tracer)

	// Middlewares will be wrapped in order
	middlewares := []middleware.Interface{
		tracerMiddleware,
		authMiddleware,
	}

	server, err := server.NewServer(conf.Logger, conf.HTTPConfig, router, middlewares)
	if err != nil {
		return nil, fmt.Errorf("failed to create http server: %w", err)
	}

	api, err := NewAPI(conf, client, recorder)
	if err != nil {
		return nil, fmt.Errorf("failed to create influx API: %w", err)
	}

	api.Register(server.Router)
	err = recorder.RegisterVersionBuildTimestamp()
	if err != nil {
		return nil, fmt.Errorf("could not register version build timestamp: %w", err)
	}

	p := &ProxyService{
		logger:       conf.Logger,
		config:       conf,
		server:       server,
		errChan:      make(chan error, 1),
		tracerCloser: tracerCloser.Close,
	}
	p.Service = services.NewBasicService(p.start, p.run, p.stop).WithName(serviceName)
	return p, nil
}

// Addr returns the net.Addr for the configured server. This is useful in case
// it was started with port auto-selection so the port number can be retrieved.
func (p *ProxyService) Addr() net.Addr {
	return p.server.Addr()
}

func (p *ProxyService) start(_ context.Context) error {
	// the server does not listen for context canceling, so we have to start it
	// in a goroutine so we can listen for both.
	go func() {
		err := p.server.Run()
		p.errChan <- err
	}()

	return nil
}

func (p *ProxyService) run(servCtx context.Context) error {
	for {
		select {
		case <-servCtx.Done():
			return servCtx.Err()
		case err := <-p.errChan:
			return err
		}
	}
}

func (p *ProxyService) stop(_ error) error {
	p.server.Shutdown(nil)
	return p.tracerCloser()
}
