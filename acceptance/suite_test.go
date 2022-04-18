package influxtest

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ahmetalpbalkan/dlog"
	"github.com/colega/envconfig"
	influxdb "github.com/influxdata/influxdb-client-go/v2"
	influxdb_api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/suite"
)

var (
	suiteContainerLabels = map[string]string{"acceptance-suite": "influx-proxy"}
)

// These tests verify that the Influx proxy is able to take in InfluxDB line protocol,
// correctly parse it and convert it into a timeseries, and write the timeseries to
// Cortex. Several services are run to execute the tests. The InfluxDB client is used to
// send line protocol to the proxy. The proxy service accepts the line protocol, parses it,
// and writes it to the Cortex service. The Prometheus client and API are used to query
// Prometheus to verify that the line protocol was parsed and converted into the expected
// timeseries, and that the timeseries was successfully written.

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

type Suite struct {
	suite.Suite

	// cfg is filled with env values starting with ACCEPTANCE_ prefix,
	// for example, ACCEPTANCE_DOCKER_HOST for cfg.Docker.Host
	cfg struct {
		// CI should be set to true in CI env
		CI     bool
		Docker struct {
			// Host defines where docker containers run
			// This is `localhost` for local env, but in CI containers run in a dind service that runs on `docker` host.
			Host string `default:"localhost"`
			// Tag defines which version of our apps has to be tested. In local, these are built using `make build-local`,
			// while in CI those are images pushed to gcr ready to be used in production.
			Tag string `default:"local"`
			// Auth is used in CI to be able to pull images from GCR. In local we expect the images to be already built.
			Auth docker.AuthConfiguration
		}
	}

	pool    *dockertest.Pool
	network *dockertest.Network

	cortexResource      *dockertest.Resource
	influxProxyResource *dockertest.Resource

	api struct {
		influx_client influxdb.Client
		writeAPI      influxdb_api.WriteAPIBlocking
		promClient    promapi.Client
		querierClient promv1.API
	}

	suiteReady time.Time
}

// This method sets up the services that are used for these tests: cortex, the influx proxy,
// the InfluxDB client, and the Prometheus client and API.
func (s *Suite) SetupSuite() {
	t0 := time.Now()
	s.Require().NoError(envconfig.Process("ACCEPTANCE", &s.cfg))
	s.Require().NotEmpty(s.cfg.Docker.Tag, "ACCEPTANCE_DOCKER_TAG should not be empty as that would test the latest version. It should be a branch tag or `local` for local testing")

	pool, err := dockertest.NewPool("")
	s.Require().NoError(err)
	s.pool = pool
	s.Require().NoError(s.pool.Retry(s.pool.Client.Ping), "Docker daemon isn't ready")

	s.network = s.createNetwork()
	s.cortexResource = s.startCortex()
	s.influxProxyResource = s.startInfluxProxy()

	influx_write_endpoint := fmt.Sprintf("http://%s:%s/", "0.0.0.0", s.influxProxyResource.GetPort("8080/tcp"))
	influx_client := influxdb.NewClient(influx_write_endpoint, "my-token")
	write_api := influx_client.WriteAPIBlocking("my-org", "my-bucket")
	s.api.influx_client = influx_client
	s.api.writeAPI = write_api

	// Prometheus client and API for verifying that writes occurred as expected
	s.api.promClient, _ = promapi.NewClient(promapi.Config{
		Address: fmt.Sprintf("http://%s:%s/api/prom", s.cfg.Docker.Host, s.cortexResource.GetPort("9009/tcp")),
	})
	s.api.querierClient = promv1.NewAPI(s.api.promClient)

	s.suiteReady = time.Now()
	s.T().Logf("Setup complete, took %s", time.Since(t0))
}

func (s *Suite) TearDownSuite() {
	if s.T().Failed() {
		if s.cfg.CI {
			s.printLogs(s.influxProxyResource)
		} else {
			s.T().Logf("Not stopping containers to allow logs inspection")
			s.T().Logf("Use the following command to stop them:")
			s.T().Logf(`docker ps --filter "label=acceptance-suite=influx-proxy" -q | xargs docker stop | xargs docker rm`)
			return
		}
	}

	s.NoError(s.cortexResource.Close())
	s.NoError(s.influxProxyResource.Close())
	s.NoError(s.network.Close())
}

func (s *Suite) SetupTest() {
	s.T().Logf("Test started at %s", time.Now().Format(time.RFC3339Nano))
}

func (s *Suite) TearDownTest() {
	s.T().Logf("Test ended at %s", time.Now().Format(time.RFC3339Nano))
}

func (s *Suite) printLogs(res *dockertest.Resource) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	buf := &bytes.Buffer{}
	s.Assert().NoError(s.pool.Client.Logs(docker.LogsOptions{
		Context:           ctx,
		Container:         res.Container.ID,
		OutputStream:      buf,
		InactivityTimeout: 0,
		Stdout:            true,
		Stderr:            true,
		Timestamps:        true,
		RawTerminal:       true, // put everything on OutputStream
	}))

	scanner := bufio.NewScanner(dlog.NewReader(buf))
	s.T().Logf("Logs from %s", res.Container.Name)
	for scanner.Scan() {
		s.T().Logf("%s | %s", res.Container.Name, scanner.Text())
	}
}

func (s *Suite) createNetwork() *dockertest.Network {
	network, err := s.pool.CreateNetwork("acceptance-testing")
	s.Require().NoError(err)
	return network
}

func (s *Suite) startCortex() *dockertest.Resource {
	const (
		name = "cortex"
		repo = "cortexproject/cortex"
		tag  = "master-3018a54"
	)

	container := s.startContainer(&dockertest.RunOptions{
		Name:         name,
		Repository:   repo,
		Tag:          tag,
		Env:          nil,
		Cmd:          []string{"cortex", "-config.file=/etc/config/cortex-config.yaml"},
		Mounts:       []string{s.testFilePath() + "/config/cortex.yaml:/etc/config/cortex-config.yaml"},
		ExposedPorts: []string{"9009"},
		PortBindings: map[docker.Port][]docker.PortBinding{"9009/tcp": {}},
		Networks:     []*dockertest.Network{s.network},
		Labels:       suiteContainerLabels,
	}, "ready", "9009/tcp")

	return container
}

func (s *Suite) startInfluxProxy() *dockertest.Resource {
	const (
		name = "influx2cortex"
		repo = "us.gcr.io/kubernetes-dev/influx2cortex/local"
	)

	return s.startContainer(&dockertest.RunOptions{
		Name:       name,
		Repository: repo,
		Tag:        s.cfg.Docker.Tag,
		Cmd: []string{
			"/app/influx2cortex",
			"-server.http-listen-address=0.0.0.0",
			"-server.http-listen-port=8080",
			"-auth.enable=false",
			"-write-endpoint=http://cortex:9009/api/prom/push",
		},
		ExposedPorts: []string{"8080", "8081", "9095"},
		Networks:     []*dockertest.Network{s.network},
		PortBindings: map[docker.Port][]docker.PortBinding{"8080/tcp": {}, "8081/tcp": {}, "9095/tcp": {}},
		Privileged:   false,
		Auth:         s.cfg.Docker.Auth,
		Labels:       suiteContainerLabels,
	}, "healthz", "8081/tcp")
}

func (s *Suite) waitForReady(template string, args ...interface{}) {
	s.Require().NoError(
		s.pool.Retry(
			healthCheck(template, args...),
		),
	)
}

// testFilePath returns the path of this test file, without the trailing slash
func (s *Suite) testFilePath() string {
	_, testFilename, _, _ := runtime.Caller(0)
	path := testFilename[:strings.LastIndex(testFilename, "/")]
	return path
}

// healthCheck creates a health check function that will succeed once the url returns a 200 OK status
// the template provided will be formatted-f with the args
func healthCheck(template string, args ...interface{}) func() error {
	url := fmt.Sprintf(template, args...)
	return func() error {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code isn't 200 OK, is %s", resp.Status)
		}
		return nil
	}
}

// waitUntilElapsedAfterSuiteSetup waits until duration is elapsed after suite setup
// this allows dynamic wait periods depending on the amount of tests running,
// avoiding double waits when more than one test is running which in the end makes waiting period more deterministic.
func (s *Suite) waitUntilElapsedAfterSuiteSetup(duration time.Duration) {
	wait := time.Until(s.suiteReady.Add(duration))
	s.T().Logf("Waiting %s", wait)
	time.Sleep(wait)
	s.T().Log("Done waiting")
}

// startContainer will run a container with the given options, run a health check until the resource is available,
// and then return.
func (s *Suite) startContainer(runOptions *dockertest.RunOptions, healthEndpoint, healthPort string) *dockertest.Resource {
	// Make sure that there are no containers running from previous execution first
	s.Require().NoError(s.pool.RemoveContainerByName(runOptions.Name))

	image := fmt.Sprintf("%s:%s", runOptions.Repository, runOptions.Tag)
	s.T().Logf("Starting %s from %s", runOptions.Name, image)

	if runOptions.Tag == "local" {
		_, err := s.pool.Client.InspectImage(image)
		s.Require().NoError(err, "Could not find %s, have you run `make build-local`?", image)
	}

	resource, err := s.pool.RunWithOptions(runOptions)
	s.Require().NoError(err)
	s.waitForReady("http://%s:%s/"+healthEndpoint, s.cfg.Docker.Host, resource.GetPort(healthPort))
	return resource
}
