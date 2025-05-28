package influx

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/mimir-graphite/v2/pkg/errorx"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	grpcstatus "google.golang.org/grpc/status"
)

// Error codes copied from https://github.com/influxdata/influxdb/blob/569e84d4a73250f2928136e5b40452d058a149bd/kit/platform/errors/errors.go#L15-L29
const (
	EInternal            = "internal error"
	ENotImplemented      = "not implemented"
	ENotFound            = "not found"
	EConflict            = "conflict"             // action cannot be performed
	EInvalid             = "invalid"              // validation failed
	EUnprocessableEntity = "unprocessable entity" // data type is correct, but out of range
	EEmptyValue          = "empty value"
	EUnavailable         = "unavailable"
	EForbidden           = "forbidden"
	ETooManyRequests     = "too many requests"
	EUnauthorized        = "unauthorized"
	EMethodNotAllowed    = "method not allowed"
	ETooLarge            = "request too large"
)

func errorxToInfluxErrorCode(err errorx.Error) string {
	switch {
	case errors.As(err, &errorx.BadRequest{}):
		return EInvalid
	case errors.As(err, &errorx.Internal{}):
		return EInternal
	case errors.As(err, &errorx.Conflict{}):
		return EConflict
	}
	return EInternal
}

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
func (a *API) handleError(w http.ResponseWriter, r *http.Request, err error, logger log.Logger) {
	var statusCode int
	var httpErrString string
	var errx errorx.Error
	errorCode := EInternal
	switch {
	case errors.As(err, &errx):
		errorCode = errorxToInfluxErrorCode(errx)
		httpErrString = errx.Message()
		statusCode = errx.HTTPStatusCode()
		err = errx
	case errors.Is(err, context.DeadlineExceeded) || isGRPCTimeout(err):
		httpErrString = "network timeout"
		statusCode = http.StatusGatewayTimeout
		errorCode = EUnavailable
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
			errorCode = EUnavailable
		}
	default:
		httpErrString = "uncategorized error"
		statusCode = http.StatusInternalServerError
	}
	if statusCode < 500 {
		_ = level.Info(logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	} else if statusCode >= 500 {
		_ = level.Warn(logger).Log("msg", httpErrString, "response_code", statusCode, "err", tryUnwrap(err))
	}
	a.recorder.measureProxyErrors(fmt.Sprintf("%T", err))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	e := struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{
		Code:    errorCode,
		Message: httpErrString,
	}
	b, err := json.Marshal(e)
	if err != nil {
		_ = level.Warn(logger).Log("msg", "failed to marshal error response", "err", err)
		return
	}
	_, err = w.Write(b)
	if err != nil {
		_ = level.Warn(logger).Log("msg", "failed to write error response", "err", err)
		return
	}
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
