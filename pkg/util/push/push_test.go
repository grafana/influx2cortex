// SPDX-License-Identifier: AGPL-3.0-only
// Provenance-includes-location: https://github.com/cortexproject/cortex/blob/master/pkg/util/push/push_test.go
// Provenance-includes-license: Apache-2.0
// Provenance-includes-copyright: The Cortex Authors.

package push

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/middleware"

	"github.com/cortexproject/cortex/pkg/cortexpb"
)

func TestHandler_remoteWrite(t *testing.T) {
	req := createRequest(t, createPrometheusRemoteWriteProtobuf(t))
	resp := httptest.NewRecorder()
	handler := Handler(100000, nil, false, verifyWriteRequestHandler(t, cortexpb.API))
	handler.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)
}

func TestHandler_cortexWriteRequest(t *testing.T) {
	req := createRequest(t, createWriteRequestProtobuf(t, false))
	resp := httptest.NewRecorder()
	sourceIPs, _ := middleware.NewSourceIPs("SomeField", "(.*)")
	handler := Handler(100000, sourceIPs, false, verifyWriteRequestHandler(t, cortexpb.RULE))
	handler.ServeHTTP(resp, req)
	assert.Equal(t, 200, resp.Code)
}

func TestHandler_EnsureSkipLabelNameValidationBehaviour(t *testing.T) {
	tests := []struct {
		name                                      string
		allowSkipLabelNameValidation              bool
		req                                       *http.Request
		includeAllowSkiplabelNameValidationHeader bool
		verifyReqHandler                          func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error)
		expectedStatusCode                        int
	}{
		{
			name:                         "config flag set to false means SkipLabelNameValidation is false",
			allowSkipLabelNameValidation: false,
			req:                          createRequest(t, createWriteRequestProtobufWithNonSupportedLabelNames(t, false)),
			verifyReqHandler: func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
				assert.Len(t, request.Timeseries, 1)
				assert.Equal(t, "a-label", request.Timeseries[0].Labels[0].Name)
				assert.Equal(t, "value", request.Timeseries[0].Labels[0].Value)
				assert.Equal(t, cortexpb.RULE, request.Source)
				assert.False(t, request.SkipLabelNameValidation)
				cleanup()
				return &cortexpb.WriteResponse{}, nil
			},
			includeAllowSkiplabelNameValidationHeader: true,
			expectedStatusCode:                        http.StatusOK,
		},
		{
			name:                         "config flag set to false means SkipLabelNameValidation is always false even if write requests sets it to true",
			allowSkipLabelNameValidation: false,
			req:                          createRequest(t, createWriteRequestProtobufWithNonSupportedLabelNames(t, true)),
			verifyReqHandler: func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
				assert.Len(t, request.Timeseries, 1)
				assert.Equal(t, "a-label", request.Timeseries[0].Labels[0].Name)
				assert.Equal(t, "value", request.Timeseries[0].Labels[0].Value)
				assert.Equal(t, cortexpb.RULE, request.Source)
				assert.False(t, request.SkipLabelNameValidation)
				cleanup()
				return &cortexpb.WriteResponse{}, nil
			},
			includeAllowSkiplabelNameValidationHeader: true,
			expectedStatusCode:                        http.StatusOK,
		},
		{
			name:                         "config flag set to true but write request set to false means SkipLabelNameValidation is false",
			allowSkipLabelNameValidation: true,
			req:                          createRequest(t, createWriteRequestProtobufWithNonSupportedLabelNames(t, false)),
			verifyReqHandler: func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
				assert.Len(t, request.Timeseries, 1)
				assert.Equal(t, "a-label", request.Timeseries[0].Labels[0].Name)
				assert.Equal(t, "value", request.Timeseries[0].Labels[0].Value)
				assert.Equal(t, cortexpb.RULE, request.Source)
				assert.False(t, request.SkipLabelNameValidation)
				cleanup()
				return &cortexpb.WriteResponse{}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:                         "config flag set to true and write request set to true means SkipLabelNameValidation is true",
			allowSkipLabelNameValidation: true,
			req:                          createRequest(t, createWriteRequestProtobufWithNonSupportedLabelNames(t, true)),
			verifyReqHandler: func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
				assert.Len(t, request.Timeseries, 1)
				assert.Equal(t, "a-label", request.Timeseries[0].Labels[0].Name)
				assert.Equal(t, "value", request.Timeseries[0].Labels[0].Value)
				assert.Equal(t, cortexpb.RULE, request.Source)
				assert.True(t, request.SkipLabelNameValidation)
				cleanup()
				return &cortexpb.WriteResponse{}, nil
			},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:                         "config flag set to true and write request set to true but header not sent means SkipLabelNameValidation is false",
			allowSkipLabelNameValidation: true,
			req:                          createRequest(t, createWriteRequestProtobufWithNonSupportedLabelNames(t, true)),
			verifyReqHandler: func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
				assert.Len(t, request.Timeseries, 1)
				assert.Equal(t, "a-label", request.Timeseries[0].Labels[0].Name)
				assert.Equal(t, "value", request.Timeseries[0].Labels[0].Value)
				assert.Equal(t, cortexpb.RULE, request.Source)
				assert.False(t, request.SkipLabelNameValidation)
				cleanup()
				return &cortexpb.WriteResponse{}, nil
			},
			includeAllowSkiplabelNameValidationHeader: true,
			expectedStatusCode:                        http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			handler := Handler(100000, nil, tc.allowSkipLabelNameValidation, tc.verifyReqHandler)
			if !tc.includeAllowSkiplabelNameValidationHeader {
				tc.req.Header.Set(SkipLabelNameValidationHeader, "true")
			}
			handler.ServeHTTP(resp, tc.req)
			assert.Equal(t, tc.expectedStatusCode, resp.Code)
		})
	}
}

func verifyWriteRequestHandler(t *testing.T, expectSource cortexpb.WriteRequest_SourceEnum) func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
	t.Helper()
	return func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
		assert.Len(t, request.Timeseries, 1)
		assert.Equal(t, "__name__", request.Timeseries[0].Labels[0].Name)
		assert.Equal(t, "foo", request.Timeseries[0].Labels[0].Value)
		assert.Equal(t, expectSource, request.Source)
		assert.False(t, request.SkipLabelNameValidation)
		cleanup()
		return &cortexpb.WriteResponse{}, nil
	}
}

func createRequest(t testing.TB, protobuf []byte) *http.Request {
	t.Helper()
	inoutBytes := snappy.Encode(nil, protobuf)
	req, err := http.NewRequest("POST", "http://localhost/", bytes.NewReader(inoutBytes))
	require.NoError(t, err)
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	return req
}

func createPrometheusRemoteWriteProtobuf(t testing.TB) []byte {
	t.Helper()
	input := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "foo"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC).UnixNano()},
				},
			},
		},
	}
	inoutBytes, err := input.Marshal()
	require.NoError(t, err)
	return inoutBytes
}

func createWriteRequestProtobuf(t *testing.T, skipLabelNameValidation bool) []byte {
	t.Helper()
	ts := cortexpb.PreallocTimeseries{
		TimeSeries: &cortexpb.TimeSeries{
			Labels: []cortexpb.LabelAdapter{
				{Name: "__name__", Value: "foo"},
			},
			Samples: []cortexpb.Sample{
				{Value: 1, TimestampMs: time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC).UnixNano()},
			},
		},
	}
	input := cortexpb.WriteRequest{
		Timeseries:              []cortexpb.PreallocTimeseries{ts},
		Source:                  cortexpb.RULE,
		SkipLabelNameValidation: skipLabelNameValidation,
	}
	inoutBytes, err := input.Marshal()
	require.NoError(t, err)
	return inoutBytes
}

func createWriteRequestProtobufWithNonSupportedLabelNames(t *testing.T, skipLabelNameValidation bool) []byte {
	t.Helper()
	ts := cortexpb.PreallocTimeseries{
		TimeSeries: &cortexpb.TimeSeries{
			Labels: []cortexpb.LabelAdapter{
				{Name: "a-label", Value: "value"}, // a-label does not comply with regex [a-zA-Z_:][a-zA-Z0-9_:]*
			},
			Samples: []cortexpb.Sample{
				{Value: 1, TimestampMs: time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC).UnixNano()},
			},
		},
	}
	input := cortexpb.WriteRequest{
		Timeseries:              []cortexpb.PreallocTimeseries{ts},
		Source:                  cortexpb.RULE,
		SkipLabelNameValidation: skipLabelNameValidation,
	}
	inoutBytes, err := input.Marshal()
	require.NoError(t, err)
	return inoutBytes
}

func BenchmarkPushHandler(b *testing.B) {
	protobuf := createPrometheusRemoteWriteProtobuf(b)
	buf := bytes.NewBuffer(snappy.Encode(nil, protobuf))
	req := createRequest(b, protobuf)
	pushFunc := func(ctx context.Context, request *cortexpb.WriteRequest, cleanup func()) (response *cortexpb.WriteResponse, err error) {
		cleanup()
		return &cortexpb.WriteResponse{}, nil
	}
	handler := Handler(100000, nil, false, pushFunc)
	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		req.Body = bufCloser{Buffer: buf} // reset Body so it can be read each time round the loop
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		assert.Equal(b, 200, resp.Code)
	}
}

// Implements both io.ReadCloser required by http.NewRequest and BytesBuffer used by push handler.
type bufCloser struct {
	*bytes.Buffer
}

func (bufCloser) Close() error                 { return nil }
func (n bufCloser) BytesBuffer() *bytes.Buffer { return n.Buffer }
