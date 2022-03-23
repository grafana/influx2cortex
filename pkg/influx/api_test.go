package influx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/log"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/grafana/influx2cortex/pkg/remotewrite/remotewritemock"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleSeriesPush(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		data            string
		expectedCode    int
		remoteWriteMock func() *remotewritemock.Client
		recorderMock    func() *MockRecorder
	}{
		{
			name:         "POST",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &cortexpb.WriteRequest{
					Timeseries: []cortexpb.PreallocTimeseries{
						{
							TimeSeries: &cortexpb.TimeSeries{
								Labels: []cortexpb.LabelAdapter{
									{Name: "__name__", Value: "measurement_f1"},
									{Name: "t1", Value: "v1"},
								},
								Samples: []cortexpb.Sample{
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
		},
		{
			name:         "POST with precision",
			url:          "/write?precision=ns",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusNoContent,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, &cortexpb.WriteRequest{
					Timeseries: []cortexpb.PreallocTimeseries{
						{
							TimeSeries: &cortexpb.TimeSeries{
								Labels: []cortexpb.LabelAdapter{
									{Name: "__name__", Value: "measurement_f1"},
									{Name: "t1", Value: "v1"},
								},
								Samples: []cortexpb.Sample{
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
		},
		{
			name:         "invalid parsing error handling",
			url:          "/write",
			data:         "measurement,t1=v1 f1= 1465839830100400200",
			expectedCode: http.StatusBadRequest,
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
		},
		{
			name:         "invalid query params",
			url:          "/write?precision=?",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusBadRequest,
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
		},
		{
			name:         "internal server error",
			url:          "/write",
			data:         "measurement,t1=v1 f1=2 1465839830100400200",
			expectedCode: http.StatusInternalServerError,
			remoteWriteMock: func() *remotewritemock.Client {
				remoteWriteMock := &remotewritemock.Client{}
				remoteWriteMock.On("Write", mock.Anything, mock.Anything).
					Return(errorx.Internal{})
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, bytes.NewReader([]byte(tt.data)))
			rec := httptest.NewRecorder()
			logger := log.NewNopLogger()
			api, err := NewAPI(logger, tt.remoteWriteMock(), tt.recorderMock())
			require.NoError(t, err)

			api.handleSeriesPush(rec, req)
			assert.Equal(t, tt.expectedCode, rec.Code)
		})
	}
}
