package influx

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/grafana/influx2cortex/pkg/remotewrite"
	"github.com/grafana/influx2cortex/pkg/remotewrite/remotewritemock"
	"github.com/grafana/influx2cortex/pkg/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/user"
)

func TestAuthentication(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		data          string
		enableAuth    bool
		orgID         string
		expectedOrgID string
		expectedCode  int
		expectedErr   error
	}{
		{
			name:          "test auth enabled valid org ID",
			url:           "/write",
			data:          "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:    true,
			orgID:         "valid",
			expectedOrgID: "valid",
			expectedCode:  http.StatusNoContent,
			expectedErr:   nil,
		},
		{
			name:          "test auth enabled invalid org ID",
			url:           "/write",
			data:          "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:    true,
			orgID:         "",
			expectedOrgID: "",
			expectedCode:  http.StatusUnauthorized,
			expectedErr:   errorx.BadRequest{},
		},
		{
			name:          "test auth disabled",
			url:           "/write",
			data:          "measurement,t1=v1 f1=2 1465839830100400200",
			enableAuth:    false,
			orgID:         "",
			expectedOrgID: "fake",
			expectedCode:  http.StatusNoContent,
			expectedErr:   nil,
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
				Registerer:        prometheus.NewRegistry(),
			}

			remoteWriteMock := &remotewritemock.Client{}
			remoteWriteMock.On("Write", mock.Anything, mock.Anything).
				Return(tt.expectedErr).Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				orgID, err := user.ExtractOrgID(ctx)
				require.NoError(t, err)
				require.Equal(t, orgID, tt.expectedOrgID)
			})

			server, err := newProxyWithClient(apiConfig, remoteWriteMock)
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

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, tt.expectedCode, resp.StatusCode)
		})
	}
}