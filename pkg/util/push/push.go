// SPDX-License-Identifier: AGPL-3.0-only
// Provenance-includes-location: https://github.com/cortexproject/cortex/blob/master/pkg/util/push/push.go
// Provenance-includes-license: Apache-2.0
// Provenance-includes-copyright: The Cortex Authors.

package push

import (
	"context"
	"net/http"
	"sync"

	"github.com/cortexproject/cortex/pkg/cortexpb"
	"github.com/go-kit/log/level"
	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/middleware"

	"github.com/grafana/influx2cortex/pkg/util"
	"github.com/grafana/influx2cortex/pkg/util/log"
)

// Func defines the type of the push. It is similar to http.HandlerFunc.
type Func func(ctx context.Context, req *cortexpb.WriteRequest, cleanup func()) (*cortexpb.WriteResponse, error)

// Wrap a slice in a struct so we can store a pointer in sync.Pool
type bufHolder struct {
	buf []byte
}

var bufferPool = sync.Pool{
	New: func() interface{} { return &bufHolder{buf: make([]byte, 256*1024)} },
}

const SkipLabelNameValidationHeader = "X-Cortex-SkipLabelNameValidation"

// Handler is a http.Handler which accepts WriteRequests.
func Handler(
	maxRecvMsgSize int,
	sourceIPs *middleware.SourceIPExtractor,
	allowSkipLabelNameValidation bool,
	push Func,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := log.WithContext(ctx, log.Logger)
		if sourceIPs != nil {
			source := sourceIPs.Get(r)
			if source != "" {
				ctx = util.AddSourceIPsToOutgoingContext(ctx, source)
				logger = log.WithSourceIPs(source, logger)
			}
		}
		bufHolder := bufferPool.Get().(*bufHolder)
		var req cortexpb.PreallocWriteRequest
		buf, err := util.ParseProtoReader(ctx, r.Body, int(r.ContentLength), maxRecvMsgSize, bufHolder.buf, &req, util.RawSnappy)
		if err != nil {
			level.Error(logger).Log("err", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			bufferPool.Put(bufHolder)
			return
		}
		// If decoding allocated a bigger buffer, put that one back in the pool.
		if len(buf) > len(bufHolder.buf) {
			bufHolder.buf = buf
		}

		cleanup := func() {
			cortexpb.ReuseSlice(req.Timeseries)
			bufferPool.Put(bufHolder)
		}

		if allowSkipLabelNameValidation {
			req.SkipLabelNameValidation = req.SkipLabelNameValidation && r.Header.Get(SkipLabelNameValidationHeader) == "true"
		} else {
			req.SkipLabelNameValidation = false
		}

		if req.Source == 0 {
			req.Source = cortexpb.API
		}

		if _, err := push(ctx, &req.WriteRequest, cleanup); err != nil {
			resp, ok := httpgrpc.HTTPResponseFromError(err)
			if !ok {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if resp.GetCode() != 202 {
				level.Error(logger).Log("msg", "push error", "err", err)
			}
			http.Error(w, string(resp.Body), int(resp.Code))
		}
	})
}