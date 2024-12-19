package influx

import (
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/user"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"
	"github.com/grafana/mimir-proxies/pkg/route"
	"github.com/grafana/mimir-proxies/pkg/server/middleware"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
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
	registerer.RegisterRoute("/healthz", http.HandlerFunc(a.handleHealth), http.MethodGet)
}

func NewAPI(conf ProxyConfig, client remotewrite.Client, recorder Recorder) (*API, error) {
	return &API{
		logger:              conf.Logger,
		client:              client,
		recorder:            recorder,
		maxRequestSizeBytes: conf.MaxRequestSizeBytes,
	}, nil
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
	w.WriteHeader(http.StatusOK)
}

// HandlerForInfluxLine is a http.Handler which accepts Influx Line protocol and converts it to WriteRequests.
func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "handleSeriesPush")
	defer span.Finish()

	logger := withRequestInfo(a.logger, r)
	beforeConversion := time.Now()

	ts, err, bytesRead := parseInfluxLineReader(ctx, r, a.maxRequestSizeBytes)
	span.LogKV("bytesRead", bytesRead)
	logger = log.With(logger, "bytesRead", bytesRead)
	if err != nil {
		ext.LogError(span, err)
		a.handleError(w, r, err, logger)
		return
	}

	nosMetrics := len(ts)
	logger = log.With(logger, "nosMetrics", nosMetrics)
	span.LogKV("nosMetrics", nosMetrics)

	a.recorder.measureMetricsParsed(nosMetrics)
	a.recorder.measureConversionDuration(time.Since(beforeConversion))

	// Sigh, a write API optimisation needs me to jump through hoops.
	pts := make([]mimirpb.PreallocTimeseries, 0, nosMetrics)
	for i := range ts {
		pts = append(pts, mimirpb.PreallocTimeseries{
			TimeSeries: &ts[i],
		})
	}
	rwReq := &mimirpb.WriteRequest{
		Timeseries: pts,
	}

	if err := a.client.Write(ctx, rwReq); err != nil {
		ext.LogError(span, err)
		a.handleError(w, r, err, logger)
		return
	}
	a.recorder.measureMetricsWritten(len(rwReq.Timeseries))
	span.LogKV("nosMetricsWritten", len(rwReq.Timeseries))
	statusCode := http.StatusNoContent
	_ = level.Info(logger).Log("response_code", statusCode)
	w.WriteHeader(statusCode) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}

func withRequestInfo(logger log.Logger, r *http.Request) log.Logger {
	ctx := r.Context()
	if traceID, ok := middleware.ExtractSampledTraceID(ctx); ok {
		logger = log.With(logger, "traceID", traceID)
	}
	if orgID, err := user.ExtractOrgID(ctx); err == nil {
		logger = log.With(logger, "orgID", orgID)
	}
	if userID, err := user.ExtractUserID(ctx); err == nil {
		logger = log.With(logger, "userID", userID)
	}
	return log.With(logger, "path", r.URL.EscapedPath())
}
