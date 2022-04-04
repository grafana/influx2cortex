package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/grafana/influx2cortex/pkg/server/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

func Run() error {
	var (
		serverConfig      server.Config
		enableAuth        bool
		remoteWriteConfig remotewrite.Config
	)

	// Register flags.
	flag.BoolVar(&enableAuth, "auth.enable", true, "enable X-Scope-OrgId header")
	flagext.RegisterFlags(
		&serverConfig,
		&remoteWriteConfig,
	)
	flag.Parse()

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	recorder := influx.NewRecorder(prometheus.DefaultRegisterer)

	var authMiddleware middleware.Interface
	if enableAuth {
		authMiddleware = middleware.NewHTTPAuth(logger)
	} else {
		authMiddleware = middleware.HTTPFakeAuth{}
	}

	server, err := server.NewServer(logger, serverConfig, mux.NewRouter(), []middleware.Interface{authMiddleware})
	if err != nil {
		level.Error(logger).Log("msg", "failed to start server", "err", err)
		return err
	}

	remoteWriteRecorder := remotewrite.NewRecorder("influx_proxy", prometheus.DefaultRegisterer)
	client, err := remotewrite.NewClient(remoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		level.Error(logger).Log("msg", "failed to instantiate remotewrite.API for influx2cortex", "err", err)
		return err
	}

	api, err := influx.NewAPI(logger, client, recorder)
	if err != nil {
		level.Error(logger).Log("msg", "failed to start API", "err", err)
		return err
	}

	api.Register(server.Router)
	err = recorder.RegisterVersionBuildTimestamp()
	if err != nil {
		return fmt.Errorf("could not register version build timestamp: %w", err)
	}

	return server.Run()
}

func main() {
	err := Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
