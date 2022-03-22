package influx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/pkg/errors"
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

func NewAPI(logger log.Logger, client remotewrite.Client, reg prometheus.Registerer) (*API, error) {

	recorder := NewRecorder(reg)

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
	level.Debug(a.logger).Log("msg", "successful series write", "len", len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}

// handleError tries to extract an errorx.Error from the given error, logging
// and setting the http response code as needed. All non-errorx errors are
// considered internal errors. Please do not try to fix error categorization in
// this function. All client errors should be categorized as an errorx at the
// site where they are thrown.
func (a *API) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var statusCode int
	var httpErrString string
	var errx errorx.Error
	switch {
	case errors.As(err, &errx):
		httpErrString = errx.Message()
		statusCode = errx.HTTPStatusCode()
		err = errx
	case errors.Is(err, context.DeadlineExceeded) || isGRPCTimeout(err):
		httpErrString = "network timeout"
		statusCode = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		// Note: It seems unlikely this can happen other than as a timeout, so we
		// should call it an internal error.
		httpErrString = "request cancelled"
		statusCode = http.StatusInternalServerError
	case isNetworkTimeout(err):
		if r.Body != nil {
			// Try to read 1 byte from the request body. If it fails with the same error
			// it means the timeout occurred while reading the request body, so it's a 408.
			if _, readErr := r.Body.Read([]byte{0}); isNetworkTimeout(readErr) {
				httpErrString = "response timeout"
				statusCode = http.StatusRequestTimeout
				break
			}
			httpErrString = "network timeout"
			statusCode = http.StatusGatewayTimeout
		}
	default:
		httpErrString = "uncategorized error"
		statusCode = http.StatusInternalServerError
	}
	if statusCode < 500 {
		level.Info(a.logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	} else if statusCode >= 500 {
		level.Warn(a.logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	}
	a.recorder.measureProxyErrors(fmt.Sprintf("%T", err))
	http.Error(w, httpErrString, statusCode)
}
