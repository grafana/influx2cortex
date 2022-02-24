package influx

import (
	"bytes"
	"context"
	"net/http/httptest"
	"testing"

	"github.com/cortexproject/cortex/pkg/ingester/client"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestParseInfluxLineReader(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		data           string
		expectedResult []client.TimeSeries
	}{
		{
			name: "parse simple line",
			url:  "/",
			data: "measurement,t1=v1 f1=2 100",
			expectedResult: []client.TimeSeries{
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "measurement_f1"}, {Name: "t1", Value: "v1"}},
					Samples: []client.Sample{{Value: 2, TimestampMs: 0}},
				},
			},
		},
		{
			name: "parse multiple tags",
			url:  "/",
			data: "measurement,t1=v1,t2=v2,t3=v3 f1=36 100",
			expectedResult: []client.TimeSeries{
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "measurement_f1"}, {Name: "t1", Value: "v1"}, {Name: "t2", Value: "v2"}, {Name: "t3", Value: "v3"}},
					Samples: []client.Sample{{Value: 36, TimestampMs: 0}},
				},
			},
		},
		{
			name: "parse multiple fields",
			url:  "/",
			data: "measurement,t1=v1 f1=3.0,f2=365,f3=0 100",
			expectedResult: []client.TimeSeries{
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "measurement_f1"}, {Name: "t1", Value: "v1"}},
					Samples: []client.Sample{{Value: 3, TimestampMs: 0}},
				},
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "measurement_f2"}, {Name: "t1", Value: "v1"}},
					Samples: []client.Sample{{Value: 365, TimestampMs: 0}},
				},
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "measurement_f3"}, {Name: "t1", Value: "v1"}},
					Samples: []client.Sample{{Value: 0, TimestampMs: 0}},
				},
			},
		},
		{
			name: "parse invalid chars",
			url:  "/",
			data: "*measurement,#t1?=v1 f1=0 100",
			expectedResult: []client.TimeSeries{
				{
					Labels:  []client.LabelAdapter{{Name: "__name__", Value: "_measurement_f1"}, {Name: "_t1_", Value: "v1"}},
					Samples: []client.Sample{{Value: 0, TimestampMs: 0}},
				},
			},
		},
	}
	maxSize := 100 << 10

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, bytes.NewReader([]byte(tt.data)))

			timeSeries, err := parseInfluxLineReader(context.Background(), req, maxSize)
			require.NoError(t, err)
			for i := 1; i < len(timeSeries); i++ {
				assert.Equal(t, timeSeries[i].String(), tt.expectedResult[i].String())
			}
		})
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		url  string
		data string
	}{
		{
			name: "parse invalid precision",
			url:  "/write?precision=ss", // precision must be of type "ns", "us", "ms", "s"
			data: "measurement,t1=v1 f1=2 100",
		},
		{
			name: "parse invalid field input",
			url:  "/write",
			data: "measurement,t1=v1 f1= 100", // field value is missing
		},
	}
	maxSize := 100 << 10

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.url, bytes.NewReader([]byte(tt.data)))

			_, err := parseInfluxLineReader(context.Background(), req, maxSize)
			require.Error(t, err)
		})
	}
}

func TestBatchReadCloser(t *testing.T) {
	maxSize := 100 << 10

	req := httptest.NewRequest("POST", "/write", bytes.NewReader([]byte("m,t1=v1 f1=2 100")))
	req.Header.Add("Content-Encoding", "gzip")

	_, err := batchReadCloser(req.Body, "gzip", int64(maxSize))
	require.Error(t, err)
}
