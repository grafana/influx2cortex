package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cortexproject/cortex/pkg/util/fakeauth"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/influx2cortex/pkg/influx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
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
	serverConfig.Log = logging.GoKit(logger)

	recorder := influx.NewRecorder(prometheus.DefaultRegisterer)

	httpAuthMiddleware := fakeauth.SetupAuthMiddleware(&serverConfig, enableAuth, nil)

	srv, err := server.New(serverConfig)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to start server", "err", err)
		return err
	}

	remoteWriteRecorder := remotewrite.NewRecorder("influx_proxy", prometheus.DefaultRegisterer)
	client, err := remotewrite.NewClient(remoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to instantiate remotewrite.API for influx2cortex", "err", err)
		return err
	}

	api, err := influx.NewAPI(logger, client, recorder)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to start API", "err", err)
		return err
	}

	api.Register(srv, httpAuthMiddleware)
	err = recorder.RegisterVersionBuildTimestamp()
	if err != nil {
		return fmt.Errorf("could not register version build timestamp: %w", err)
	}

	return srv.Run()
}

func main() {
	err := Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
