package influx

import (
	"flag"
	"net/http"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/cortexproject/cortex/pkg/distributor/distributorpb"
	"github.com/cortexproject/cortex/pkg/util/grpcclient"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
	"google.golang.org/grpc"
)

var ingesterClientRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "influx2cortex",
	Name:      "distributor_client_request_duration_seconds",
	Help:      "Time spent doing Distributor requests.",
	Buckets:   prometheus.ExponentialBuckets(0.001, 4, 6),
}, []string{"operation", "status_code"})

type APIConfig struct {
	DistributorEndpoint     string
	DistributorClientConfig grpcclient.Config
}

func (c *APIConfig) RegisterFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.DistributorEndpoint, "distributor.endpoint", "", "The grpc endpoint for downstream Cortex distributor.")

	c.DistributorClientConfig.RegisterFlagsWithPrefix("distributor.client", flags)
}

type API struct {
	logger   log.Logger
	client   distributorpb.DistributorClient
	recorder Recorder
}

func (a *API) Register(server *server.Server, authMiddleware middleware.Interface) {
	server.HTTP.Handle("/api/v1/push/influx/write", authMiddleware.Wrap(http.HandlerFunc(a.handleSeriesPush)))
}

func NewAPI(logger log.Logger, cfg APIConfig) (*API, error) {
	dialOpts, err := cfg.DistributorClientConfig.DialOption(grpcclient.Instrument(ingesterClientRequestDuration))
	if err != nil {
		return nil, err
	}

	level.Info(logger).Log("msg", "Creating GRPC connection to", "addr", cfg.DistributorEndpoint)
	conn, err := grpc.Dial(cfg.DistributorEndpoint, dialOpts...)
	if err != nil {
		level.Error(logger).Log("msg", "Failed to connect to server", "err", err)
		return nil, err
	}

	distClient := distributorpb.NewDistributorClient(conn)

	recorder := NewRecorder(prometheus.NewRegistry())

	return &API{
		logger:   logger,
		client:   distClient,
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

	if _, err := a.client.Push(r.Context(), rwReq); err != nil {
		handleError(w, r, a.logger, err)
		return
	}

	level.Debug(a.logger).Log("msg", "successful series write", "len", len(rwReq.Timeseries))

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
