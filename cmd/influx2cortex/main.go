package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"os"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/services"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/grafana/influx2cortex/pkg/internalserver"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
)

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	promRegisterer := prometheus.DefaultRegisterer

	proxyConfig := influx.ProxyConfig{
		Logger:     logger,
		Registerer: promRegisterer,
	}
	internalServerConfig := internalserver.ServiceConfig{}

	flagext.RegisterFlags(
		&proxyConfig,
		&internalServerConfig,
	)
	flag.Parse()

	appServices := make([]services.Service, 0)

	proxyService, err := influx.NewProxy(proxyConfig)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error instantiating influx2cortex proxy: %s\n", err)
		os.Exit(1)
	}
	appServices = append(appServices, proxyService)

	internalService, err := internalserver.NewService(internalServerConfig, proxyService.Logger)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error instantiating internal server: %s\n", err)
		os.Exit(1)
	}
	appServices = append(appServices, internalService)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	for _, service := range appServices {
		err = services.StartAndAwaitRunning(ctx, service)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error starting service: %s\n", err)
			os.Exit(1)
		}
	}

	// Look for SIGTERM and stop the server if we get it
	handler := signals.NewHandler(logging.GoKit(proxyService.Logger))
	go func() {
		handler.Loop()
		for _, service := range appServices {
			service.StopAsync()
		}
	}()

	for _, service := range appServices {
		err = service.AwaitTerminated(context.Background())
		if err != nil && !errors.Is(err, context.Canceled) {
			_, _ = fmt.Fprintf(os.Stderr, "error in service: %s\n", err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
