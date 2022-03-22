package influx

import (
	"context"
	"net"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/grafana/influx2cortex/pkg/errorx"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	grpcstatus "google.golang.org/grpc/status"
)

func tryUnwrap(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}

// handleError tries to extract an errorx.Error from the given error, logging
// and setting the http response code as needed. All non-errorx errors are
// considered internal errors. Please do not try to fix error categorization in
// this function. All client errors should be categorized as an errorx at the
// site where they are thrown.
func handleError(w http.ResponseWriter, r *http.Request, logger log.Logger, err error) {
	var statusCode int
	var httpErrString string
	var errx errorx.Error
	switch {
	case errors.As(err, &errx):
		httpErrString = errx.Message()
		statusCode = errx.HTTPStatusCode()
		err = errx
	case errors.Is(err, context.DeadlineExceeded) || isGRPCTimeout(err):
		httpErrString = "network timeout"
		statusCode = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		// Note: It seems unlikely this can happen other than as a timeout, so we
		// should call it an internal error.
		httpErrString = "request cancelled"
		statusCode = http.StatusInternalServerError
	case isNetworkTimeout(err):
		if r.Body != nil {
			// Try to read 1 byte from the request body. If it fails with the same error
			// it means the timeout occurred while reading the request body, so it's a 408.
			if _, readErr := r.Body.Read([]byte{0}); isNetworkTimeout(readErr) {
				httpErrString = "response timeout"
				statusCode = http.StatusRequestTimeout
				break
			}
			httpErrString = "network timeout"
			statusCode = http.StatusGatewayTimeout
		}
	default:
		httpErrString = "uncategorized error"
		statusCode = http.StatusInternalServerError
	}
	if statusCode < 500 {
		level.Info(logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	} else if statusCode >= 500 {
		level.Warn(logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	}
	http.Error(w, httpErrString, statusCode)
}

func isNetworkTimeout(err error) bool {
	if err == nil {
		return false
	}

	netErr, ok := errors.Cause(err).(net.Error)
	return ok && netErr.Timeout()
}

func isGRPCTimeout(err error) bool {
	s, ok := grpcstatus.FromError(errors.Cause(err))
	return ok && s.Code() == codes.DeadlineExceeded
}
