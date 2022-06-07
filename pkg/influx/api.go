package influx

import (
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/influx2cortex/pkg/route"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"
	"github.com/grafana/mimir/pkg/mimirpb"
)

type API struct {
	logger              log.Logger
	client              remotewrite.Client
	recorder            Recorder
	maxRequestSizeBytes int
}

func (a *API) Register(router *mux.Router) {
	registerer := route.NewMuxRegisterer(router)

	// Registering two write endpoints; the second is necessary to allow for compatibility with clients that hard-code the endpoint
	registerer.RegisterRoute("/api/v1/push/influx/write", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
	registerer.RegisterRoute("/api/v2/write", http.HandlerFunc(a.handleSeriesPush), http.MethodPost)
}

func NewAPI(conf ProxyConfig, client remotewrite.Client, recorder Recorder) (*API, error) {
	return &API{
		logger:              conf.Logger,
		client:              client,
		recorder:            recorder,
		maxRequestSizeBytes: conf.MaxRequestSizeBytes,
	}, nil
}

// HandlerForInfluxLine is a http.Handler which accepts Influx Line protocol and converts it to WriteRequests.
func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	beforeConversion := time.Now()

	ts, err := parseInfluxLineReader(r.Context(), r, a.maxRequestSizeBytes)
	if err != nil {
		a.handleError(w, r, err)
		return
	}

	a.recorder.measureMetricsParsed(len(ts))
	a.recorder.measureConversionDuration(time.Since(beforeConversion))

	// Sigh, a write API optimisation needs me to jump through hoops.
	pts := make([]mimirpb.PreallocTimeseries, 0, len(ts))
	for i := range ts {
		pts = append(pts, mimirpb.PreallocTimeseries{
			TimeSeries: &ts[i],
		})
	}
	rwReq := &mimirpb.WriteRequest{
		Timeseries: pts,
	}

	if err := a.client.Write(r.Context(), rwReq); err != nil {
		a.handleError(w, r, err)
		return
	}
	a.recorder.measureMetricsWritten(len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
