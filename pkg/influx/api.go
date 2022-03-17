package influx

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
)

var ingesterClientRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "influx2cortex",
	Name:      "distributor_client_request_duration_seconds",
	Help:      "Time spent doing Distributor requests.",
	Buckets:   prometheus.ExponentialBuckets(0.001, 4, 6),
}, []string{"operation", "status_code"})

type API struct {
	logger   log.Logger
	client   remotewrite.Client
	recorder Recorder
}

func (a *API) Register(server *server.Server, authMiddleware middleware.Interface) {
	server.HTTP.Handle("/api/v1/push/influx/write", authMiddleware.Wrap(http.HandlerFunc(a.handleSeriesPush)))
}

func NewAPI(logger log.Logger, client remotewrite.Client) (*API, error) {

	recorder := NewRecorder(prometheus.NewRegistry())

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
		a.recorder.measureMetricsRejected(len(ts))
		handleError(w, r, a.logger, err)
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
		if errors.As(err, &errorx.RateLimited{}) {
			level.Warn(a.logger).Log("msg", "too many requests", err, err)
			http.Error(w, fmt.Sprintf("too many requests: %s", err), http.StatusTooManyRequests)
			return
		}

		level.Error(a.logger).Log("msg", "failed to push metric data", err, err)
		http.Error(w, "failed to push metric data", http.StatusInternalServerError)
		return
	}

	level.Debug(a.logger).Log("msg", "successful series write", "len", len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
