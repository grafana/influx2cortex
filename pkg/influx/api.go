package influx

import (
	"net/http"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
)

type API struct {
	logger   log.Logger
	client   remotewrite.Client
	recorder Recorder
}

func (a *API) Register(server *server.Server, authMiddleware middleware.Interface) {
	server.HTTP.Handle("/api/v1/push/influx/write", authMiddleware.Wrap(http.HandlerFunc(a.handleSeriesPush)))
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
	_ = level.Debug(a.logger).Log("msg", "successful series write", "len", len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
