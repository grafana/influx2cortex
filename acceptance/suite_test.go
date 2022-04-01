package influxtest

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
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
	"github.com/stretchr/testify/suite"
)

var (
	suiteContainerLabels = map[string]string{"acceptance-suite": "influx-proxy"}
)

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
		proxy_client  httpClient
	}

	suiteReady time.Time
}

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

	influx_client := influxdb.NewClient("http://localhost:8086", "my-token")
	write_api := influx_client.WriteAPIBlocking("my-org", "my-bucket")
	s.api.influx_client = influx_client
	s.api.writeAPI = write_api

	s.api.proxy_client = httpClient{
		endpoint:    fmt.Sprintf("http://%s:%s/api/prom/push", s.cfg.Docker.Host, s.influxProxyResource.GetPort("8080/tcp")),
		http_client: &http.Client{},
	}

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

	return s.startContainer(&dockertest.RunOptions{
		Name:         name,
		Repository:   repo,
		Tag:          tag,
		Env:          nil,
		Cmd:          []string{"cortex", "-config.file=/etc/config/cortex-config.yaml"},
		Mounts:       []string{s.testFilePath() + "/../operations/jsonnet/lib/cortex/cortex.yaml:/etc/config/cortex-config.yaml"},
		ExposedPorts: []string{"9009"},
		PortBindings: map[docker.Port][]docker.PortBinding{"9009/tcp": {}},
		Networks:     []*dockertest.Network{s.network},
		Labels:       suiteContainerLabels,
	}, "ready", "9009/tcp")
}

func (s *Suite) startInfluxProxy() *dockertest.Resource {
	const (
		name = "influx2cortex"
		repo = "us.gcr.io/kubernetes-dev/influx2cortex"
	)

	fmt.Println("Port bindings: ", map[docker.Port][]docker.PortBinding{"8080/tcp": {}, "8081/tcp": {}})
	return s.startContainer(&dockertest.RunOptions{
		Name:       name,
		Repository: repo,
		Tag:        s.cfg.Docker.Tag,
		Cmd: []string{
			"/app/influx2cortex",
			"-server.http-listen-address=localhost",
			"-server.http-listen-port=8080",
			"-auth.enable=false",
			"-write-endpoint=http://localhost:8888/api/prom/push",
		},
		ExposedPorts: []string{"8080", "8081"},
		Networks:     []*dockertest.Network{s.network},
		PortBindings: map[docker.Port][]docker.PortBinding{"8080/tcp": {}, "8081/tcp": {}},
		Privileged:   false,
		Auth:         s.cfg.Docker.Auth,
		Labels:       suiteContainerLabels,
	}, "healthz", "8080/tcp")
}

func (s *Suite) waitForReady(template string, args ...interface{}) {
	fmt.Println("in waitForReady")
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
	fmt.Println("in healthcheck")
	fmt.Println("url: ", url)
	return func() error {
		resp, err := http.Get(url)
		fmt.Println("Resp from get: ", resp)
		fmt.Println("Err from get: ", err)
		if err != nil {
			return err
		}
		fmt.Println("response code: ", resp.StatusCode)
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
	fmt.Println("in waitUntilElapsedAfterSuiteSetup")
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
	fmt.Println("starting wait for ready")
	s.waitForReady("http://%s:%s/"+healthEndpoint, s.cfg.Docker.Host, resource.GetPort(healthPort))
	fmt.Println("done with wait for ready")
	return resource
}

type httpClient struct {
	endpoint    string
	http_client *http.Client
}

func (pc httpClient) post(ctx context.Context, path string, orgId string, body io.Reader) (statusCode int, respBody []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, "POST", pc.endpoint+path, body)
	req.Header.Set("X-Scope-OrgID", orgId)
	if err != nil {
		return 0, nil, err
	}
	resp, err := pc.http_client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, _ = io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, nil
}
