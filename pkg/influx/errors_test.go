package influx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/grafana/influx2cortex/pkg/remotewrite/remotewritemock"
	"github.com/stretchr/testify/require"
)

func TestHandleError(t *testing.T) {
	tests := map[string]struct {
		req            *http.Request
		err            error
		expectedStatus int
		recorderMock   func() *MockRecorder
	}{
		"bad request": {
			req:            httptest.NewRequest("GET", "/write", strings.NewReader("")),
			err:            errorx.BadRequest{Msg: "test", Err: fmt.Errorf("new test error")},
			expectedStatus: http.StatusBadRequest,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "errorx.BadRequest").Return(nil)
				return recorderMock
			},
		},
		"context canceled": {
			req:            httptest.NewRequest("GET", "/write", strings.NewReader("")),
			err:            context.Canceled,
			expectedStatus: http.StatusInternalServerError,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "*errors.errorString").Return(nil)
				return recorderMock
			},
		},
		"deadline exceeded": {
			req:            httptest.NewRequest("GET", "/write", strings.NewReader("")),
			err:            context.DeadlineExceeded,
			expectedStatus: http.StatusGatewayTimeout,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "context.deadlineExceededError").Return(nil)
				return recorderMock
			},
		},
		"network timeout": {
			req:            httptest.NewRequest("GET", "/write", &mockReader{&mockNetworkError{}}),
			err:            &mockNetworkError{},
			expectedStatus: http.StatusRequestTimeout,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "*influx.mockNetworkError").Return(nil)
				return recorderMock
			},
		},
		"network timeout with body": {
			req:            httptest.NewRequest("GET", "/write", strings.NewReader("body")),
			err:            &mockNetworkError{},
			expectedStatus: http.StatusGatewayTimeout,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "*influx.mockNetworkError").Return(nil)
				return recorderMock
			},
		},
		"default error": {
			req:            httptest.NewRequest("GET", "/write", strings.NewReader("")),
			err:            fmt.Errorf("default error"),
			expectedStatus: http.StatusInternalServerError,
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureProxyErrors", "*errors.errorString").Return(nil)
				return recorderMock
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			remoteWriteMock := &remotewritemock.Client{}
			conf := ProxyConfig{
				Logger: log.NewNopLogger(),
			}
			api, err := NewAPI(conf, remoteWriteMock, tt.recorderMock())
			require.NoError(t, err)

			api.handleError(recorder, tt.req, tt.err)
			require.Equal(t, tt.expectedStatus, recorder.Code)
		})
	}
}

// Mock of Golang's internal poll.TimeoutError
type mockNetworkError struct{}

func (e *mockNetworkError) Error() string   { return "network error" }
func (e *mockNetworkError) Timeout() bool   { return true }
func (e *mockNetworkError) Temporary() bool { return true }

type mockReader struct {
	err error
}

func (r *mockReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
