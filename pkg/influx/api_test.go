package influx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/mimir-proxies/pkg/errorx"
	"github.com/grafana/mimir-proxies/pkg/remotewrite/remotewritemock"
	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleSeriesPush(t *testing.T) {
	tests := []struct {
		name                string
		url                 string
		data                string
		expectedCode        int
		expectJsonBody      string
		remoteWriteMock     func() *remotewritemock.Client
		recorderMock        func() *MockRecorder
		maxRequestSizeBytes int
		maxSampleAgeSeconds int64
	}{
		{
			name:         "POST",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "measurement_f1"},
									{Name: "__proxy_source__", Value: "influx"},
									{Name: "t1", Value: "v1"},
								},
								Samples: []mimirpb.Sample{
									{Value: 2, TimestampMs: 1465839830100},
								},
							},
						},
					},
				}).Return(nil)
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 1).Return(nil)
				recorderMock.On("measureMetricsWritten", 1).Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)
				return recorderMock
			},
			maxRequestSizeBytes: DefaultMaxRequestSizeBytes,
		},
		{
			name:         "POST with precision",
			url:          "/write?precision=ns",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "measurement_f1"},
									{Name: "__proxy_source__", Value: "influx"},
									{Name: "t1", Value: "v1"},
								},
								Samples: []mimirpb.Sample{
									{Value: 2, TimestampMs: 1465839830100},
								},
							},
						},
					},
				}).Return(nil)
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 1).Return(nil)
				recorderMock.On("measureMetricsWritten", 1).Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)
				return recorderMock
			},
			maxRequestSizeBytes: DefaultMaxRequestSizeBytes,
		},
		{
			name:         "invalid parsing error handling",
			url:          "/write",
			data:         "measurement,t1=v1 f1= 1465839830100400200",
			expectedCode: http.StatusBadRequest,
			expectJsonBody: `{
				"code": "invalid",
				"message": "error parsing points"
			}`,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, mock.Anything).
					Return(errorx.BadRequest{})
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 0).Return(nil)
				recorderMock.On("measureMetricsWritten", 0).Return(nil)
				recorderMock.On("measureProxyErrors", "errorx.BadRequest").Return(nil)
				recorderMock.On("measureConversionDuration", 0).Return(nil)
				return recorderMock
			},
			maxRequestSizeBytes: DefaultMaxRequestSizeBytes,
		},
		{
			name:         "invalid query params",
			url:          "/write?precision=?",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusBadRequest,
			expectJsonBody: `{
				"code": "invalid",
				"message": "precision supplied is not valid: ?"
			}`,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, mock.Anything).
					Return(errorx.BadRequest{})
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 0).Return(nil)
				recorderMock.On("measureMetricsWritten", 0).Return(nil)
				recorderMock.On("measureProxyErrors", "errorx.BadRequest").Return(nil)
				recorderMock.On("measureConversionDuration", 0).Return(nil)
				return recorderMock
			},
			maxRequestSizeBytes: DefaultMaxRequestSizeBytes,
		},
		{
			name:         "internal server error",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusInternalServerError,
			expectJsonBody: `{
				"code": "internal error",
				"message": "some error message"
			}`,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, mock.Anything).
					Return(errorx.Internal{Msg: "some error message"})
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 1).Return(nil)
				recorderMock.On("measureMetricsWritten", 0).Return(nil)
				recorderMock.On("measureProxyErrors", "errorx.Internal").Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)
				return recorderMock
			},
			maxRequestSizeBytes: DefaultMaxRequestSizeBytes,
		},
		{
			name:         "max batch size violated",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 0123456789",
			expectedCode: http.StatusBadRequest,
			expectJsonBody: `{
				"code": "invalid",
				"message": "problem reading body"
			}`,
			remoteWriteMock: func() *remotewritemock.Client {
				return &remotewritemock.Client{}
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 0).Return(nil)
				recorderMock.On("measureMetricsWritten", 0).Return(nil)
				recorderMock.On("measureProxyErrors", "errorx.BadRequest").Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)

				return recorderMock
			},
			maxRequestSizeBytes: 8,
		},
		{
			name:         "max sample age passed",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1680580378000000000",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{
						{
							TimeSeries: &mimirpb.TimeSeries{
								Labels: []mimirpb.LabelAdapter{
									{Name: "__name__", Value: "measurement_f1"},
									{Name: "__proxy_source__", Value: "influx"},
									{Name: "t1", Value: "v1"},
								},
								Samples: []mimirpb.Sample{
									{Value: 2, TimestampMs: 1680580378000},
								},
							},
						},
					},
				}).Return(nil)
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 1).Return(nil)
				recorderMock.On("measureMetricsWritten", 1).Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)

				return recorderMock
			},
			maxSampleAgeSeconds: 4000000000,
		},
		{
			name:         "max sample age violated",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &mimirpb.WriteRequest{
					Timeseries: []mimirpb.PreallocTimeseries{},
				}).Return(nil)
				return remoteWriteMock
			},
			recorderMock: func() *MockRecorder {
				recorderMock := &MockRecorder{}
				recorderMock.On("measureMetricsParsed", 1).Return(nil)
				recorderMock.On("measureMetricsDropped", 1).Return(nil)
				recorderMock.On("measureMetricsWritten", 0).Return(nil)
				recorderMock.On("measureConversionDuration", mock.MatchedBy(func(duration time.Duration) bool { return duration > 0 })).Return(nil)

				return recorderMock
			},
			maxSampleAgeSeconds: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, bytes.NewReader([]byte(tt.data)))
			rec := httptest.NewRecorder()
			logger := log.NewNopLogger()
			conf := ProxyConfig{
				Logger:              logger,
				MaxRequestSizeBytes: tt.maxRequestSizeBytes,
				MaxSampleAgeSeconds: tt.maxSampleAgeSeconds,
			}
			api, err := NewAPI(conf, tt.remoteWriteMock(), tt.recorderMock())
			require.NoError(t, err)

			api.handleSeriesPush(rec, req)
			assert.Equal(t, tt.expectedCode, rec.Code)
			if tt.expectJsonBody != "" {
				assert.Equal(t, []string{"application/json; charset=utf-8"}, rec.Result().Header["Content-Type"])
				assert.JSONEq(t, tt.expectJsonBody, rec.Body.String())
			} else {
				assert.Empty(t, rec.Body.String())
			}

		})
	}
}
