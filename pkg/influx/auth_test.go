package influx

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/user"
)

func TestAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		data         string
		enableAuth   bool
		orgID        string
		expectedCode int
	}{
		{
			name:         "test auth enabled valid org ID",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:   true,
			orgID:        "valid",
			expectedCode: http.StatusNoContent,
		},
		{
			name:         "test auth enabled invalid org ID",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:   true,
			orgID:        "",
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "test auth disabled",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:   false,
			orgID:        "fake",
			expectedCode: http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			serverConfig := server.Config{
				HTTPListenAddress: "127.0.0.1",
				HTTPListenPort:    0, // Request system available port
			}
			apiConfig := ProxyConfig{
				HTTPConfig:        serverConfig,
				EnableAuth:        tt.enableAuth,
				RemoteWriteConfig: remotewrite.Config{},
				Logger:            log.NewNopLogger(),
			}

			server, err := NewProxy(apiConfig)
			require.NoError(t, err)

			go func() {
				require.NoError(t, server.Run())
			}()

			defer server.Shutdown(nil)

			url := fmt.Sprintf("http://%s/api/v1/push/influx/write", server.Addr())
			req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("measurement,t1=v1 f1=2 1465839830100400200")))
			require.NoError(t, err)
			req = req.WithContext(user.InjectOrgID(req.Context(), tt.orgID))
			err = user.InjectOrgIDIntoHTTPRequest(req.Context(), req)
			require.NoError(t, err)
			require.Equal(t, req.Header.Get(user.OrgIDHeaderName), tt.orgID)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}

func NewMockRecorder() *MockRecorder {
	recorderMock := &MockRecorder{}
	recorderMock.On("measureMetricsParsed", 1).Return(nil)
	recorderMock.On("measureMetricsWritten", 1).Return(nil)
	recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)
	return recorderMock
}
