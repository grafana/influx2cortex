package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/influx2cortex/pkg/influx"
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

	err := influx.Run(conf)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
