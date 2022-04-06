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
	conf := influx.ProxyConfig{}
	flag.BoolVar(&conf.EnableAuth, "auth.enable", true, "require X-Scope-OrgId header")
	flagext.RegisterFlags(
		&conf.HTTPConfig,
		&conf.RemoteWriteConfig,
	)
	flag.Parse()

	conf.Logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))

	server, err := influx.NewProxy(conf)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error instantiating influx2cortex proxy: %s", err)
		os.Exit(1)
	}

	if err := server.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
