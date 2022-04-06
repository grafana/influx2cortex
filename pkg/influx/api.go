package influx

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/route"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/grafana/influx2cortex/pkg/server/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
)

type API struct {
	logger   log.Logger
	client   remotewrite.Client
	recorder Recorder
}

func (a *API) Register(router *mux.Router) {
	registerer := route.NewMuxRegisterer(router)
	registerer.RegisterRoute("/api/v1/push/influx/write", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
}

func NewAPI(logger log.Logger, client remotewrite.Client, recorder Recorder) (*API, error) {
	return &API{
		logger:   logger,
		client:   client,
		recorder: recorder,
	}, nil
}

// HandlerForInfluxLine is a http.Handler which accepts Influx Line protocol and converts it to WriteRequests.
func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	maxSize := 100 << 10 // TODO: Make this a CLI flag. 100KB for now.

	beforeConversion := time.Now()

	ts, err := parseInfluxLineReader(r.Context(), r, maxSize)
	if err != nil {
		a.handleError(w, r, err)
		return
	}
	a.recorder.measureMetricsParsed(len(ts))
	a.recorder.measureConversionDuration(time.Since(beforeConversion))

	// Sigh, a write API optimisation needs me to jump through hoops.
	pts := make([]cortexpb.PreallocTimeseries, 0, len(ts))
	for i := range ts {
		pts = append(pts, cortexpb.PreallocTimeseries{
			TimeSeries: &ts[i],
		})
	}
	rwReq := &cortexpb.WriteRequest{
		Timeseries: pts,
	}

	if err := a.client.Write(r.Context(), rwReq); err != nil {
		a.handleError(w, r, err)
		return
	}
	a.recorder.measureMetricsWritten(len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}

// ProxyConfig holds objects needed to start running an influx2cortex proxy
// server.
type ProxyConfig struct {
	HTTPConfig        server.Config
	EnableAuth        bool
	RemoteWriteConfig remotewrite.Config
	Logger            log.Logger
}

// NewProxy creates the influx API server with the given config options. It
// returns the HTTP server that is ready to Run.
func NewProxy(conf ProxyConfig) (*server.Server, error) {
	recorder := NewRecorder(prometheus.DefaultRegisterer)

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

	remoteWriteRecorder := remotewrite.NewRecorder("influx_proxy", prometheus.DefaultRegisterer)
	client, err := remotewrite.NewClient(conf.RemoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create remotewrite.API: %w", err)
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

	// Look for SIGTERM and stop the server if we get it
	handler := signals.NewHandler(logging.GoKit(conf.Logger))
	go func() {
		handler.Loop()
		server.Shutdown(nil)
	}()

	return server, nil
}
