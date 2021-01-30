package influx

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/cortexproject/cortex/pkg/ingester/client"
	"github.com/cortexproject/cortex/pkg/util/grpcclient"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
	"google.golang.org/grpc"
)

var ingesterClientRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "flood",
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
	logger log.Logger
	client client.PushOnlyIngesterClient
}

func (a *API) Register(server *server.Server, authMiddleware middleware.Interface) {
	server.HTTP.Handle("/api/v1/push/influx/write", authMiddleware.Wrap(http.HandlerFunc(a.handleSeriesPush)))
}

func NewAPI(logger log.Logger, cfg APIConfig) (*API, error) {
	dialOpts, err := cfg.DistributorClientConfig.DialOption(grpcclient.Instrument(ingesterClientRequestDuration))
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(cfg.DistributorEndpoint, dialOpts...)
	if err != nil {
		return nil, err
	}

	distClient := client.NewPushOnlyIngesterClient(conn)

	return &API{
		logger: logger,
		client: distClient,
	}, nil
}

// HandlerForInfluxLine is a http.Handler which accepts Influx Line protocol and converts it to WriteRequests.
func (a *API) handleSeriesPush(w http.ResponseWriter, r *http.Request) {
	maxSize := 100 << 10 // TODO: Make this a CLI flag. 100KB for now.

	ts, err := parseInfluxLineReader(r.Context(), r, maxSize)
	if err != nil {
		fmt.Println("error decoding line protocol data", err)
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	// Sigh, a write API optimisation needs me to jump through hoops.
	pts := make([]client.PreallocTimeseries, 0, len(ts))
	for i := range ts {
		pts = append(pts, client.PreallocTimeseries{
			TimeSeries: &ts[i],
		})
	}

	rwReq := &client.WriteRequest{
		Timeseries: pts,
	}

	if _, err := a.client.Push(r.Context(), rwReq); err != nil {
		resp, ok := httpgrpc.HTTPResponseFromError(err)
		if !ok {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp.GetCode() != 202 {
			level.Error(a.logger).Log("msg", "push error", "err", err)
		}
		http.Error(w, string(resp.Body), int(resp.Code))

		return
	}

	w.WriteHeader(http.StatusNoContent) // Needed for Telegraf, otherwise it tries to marshal JSON and considers the write a failure.
}
