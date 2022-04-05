package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/grafana/influx2cortex/pkg/server/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/signals"
	"github.com/weaveworks/common/logging"
)

func main() {
	conf := influx.Config{}
	flag.BoolVar(&conf.EnableAuth, "auth.enable", true, "require X-Scope-OrgId header")
	flagext.RegisterFlags(
		&conf.ServerConfig,
		&conf.RemoteWriteConfig,
	)
	flag.Parse()

	conf.Logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	conf.ServerConfig.Log = logging.GoKit(conf.Logger)

	recorder := influx.NewRecorder(prometheus.DefaultRegisterer)

	err := influx.Run(conf)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
