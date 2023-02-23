package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/services"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/grafana/influx2cortex/pkg/internalserver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
)

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	// Add caller information by skipping 3 stack frames
	logger = log.With(logger, "caller", log.Caller(3))
	logger = log.WithPrefix(logger, "ts", log.DefaultTimestampUTC)

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
		_ = level.Error(logger).Log("msg", "error instantiating influx write proxy", "error", err)
		os.Exit(1)
	}
	appServices = append(appServices, proxyService)

	internalService, err := internalserver.NewService(internalServerConfig, logger)
	if err != nil {
		_ = level.Error(logger).Log("msg", "error instantiating internal server", "error", err)
		os.Exit(1)
	}
	appServices = append(appServices, internalService)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	for _, service := range appServices {
		err = services.StartAndAwaitRunning(ctx, service)
		if err != nil {
			_ = level.Error(logger).Log(
				"msg", "error starting service",
				"service", services.DescribeService(service),
				"error", err)
			os.Exit(1)
		}
	}

	// Look for SIGTERM and stop the server if we get it
	handler := signals.NewHandler(logging.GoKit(logger))
	go func() {
		handler.Loop()
		for _, service := range appServices {
			service.StopAsync()
		}
	}()

	for _, service := range appServices {
		err = service.AwaitTerminated(context.Background())
		if err != nil && !errors.Is(err, context.Canceled) {
			_ = level.Error(logger).Log("msg", "error in service", "error", err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
