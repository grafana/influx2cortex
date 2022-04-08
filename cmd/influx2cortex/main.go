package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
)

func main() {
	conf := influx.ProxyConfig{}

	flag.BoolVar(&conf.EnableAuth, "auth.enable", true, "require X-Scope-OrgId header")
	flagext.RegisterFlags(
		&conf.HTTPConfig,
		&conf.RemoteWriteConfig,
	)
	flag.Parse()

	service, err := influx.NewProxy(conf)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error instantiating influx2cortex proxy: %s", err)
		os.Exit(1)
	}

	err = service.StartAsync(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error starting influx2cortex: %s", err)
		os.Exit(1)
	}
	err = service.AwaitRunning(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}

	// Look for SIGTERM and stop the server if we get it
	handler := signals.NewHandler(logging.GoKit(p.config.Logger))
	go func() {
		handler.Loop()
		service.StopAsync()
	}()

	err = service.AwaitTerminated(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}

	os.Exit(0)
}
