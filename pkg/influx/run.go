package influx

import (
	"flag"
	"os"

	"github.com/cortexproject/cortex/pkg/util/fakeauth"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
)

func Run() error {
	var (
		serverConfig server.Config
		enableAuth   bool
		apiConfig    APIConfig
	)

	// Register flags.
	flag.BoolVar(&enableAuth, "auth.enable", true, "enable X-Scope-OrgId header")
	flagext.RegisterFlags(
		&serverConfig,
		&apiConfig,
	)
	flag.Parse()

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	serverConfig.Log = logging.GoKit(logger)

	httpAuthMiddleware := fakeauth.SetupAuthMiddleware(&serverConfig, enableAuth, nil)

	srv, err := server.New(serverConfig)
	if err != nil {
		level.Error(logger).Log("msg", "failed to start server", "err", err)
		return err
	}

	api, err := NewAPI(logger, apiConfig)
	if err != nil {
		level.Error(logger).Log("msg", "failed to start API", "err", err)
		return err
	}

	api.Register(srv, httpAuthMiddleware)

	return srv.Run()
}
