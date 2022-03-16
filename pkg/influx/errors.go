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

type status string

const (
	StatusClientClosedRequest = 499
)

func tryUnwrap(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}

func handleError(w http.ResponseWriter, r *http.Request, logger log.Logger, err error) {
	var statusCode int
	var errx errorx.Error
	switch {
	case errors.Is(err, context.Canceled):
		statusCode = StatusClientClosedRequest
		level.Error(logger).Log("msg", "request cancelled", "err", err)
	case errors.As(err, &errx):
		switch statusCode = errx.HTTPStatusCode(); statusCode {
		case http.StatusBadRequest:
			level.Warn(logger).Log("msg", errx.Message(), "response_code", statusCode, "err", tryUnwrap(errx))
		default:
			level.Error(logger).Log("msg", errx.Message(), "response_code", statusCode, "err", tryUnwrap(errx))
		}
	case errors.Is(err, context.DeadlineExceeded) || isGRPCTimeout(err):
		statusCode = http.StatusGatewayTimeout
		level.Error(logger).Log("msg", "response timeout", "err", err)
	case isNetworkTimeout(err):
		if r.Body != nil {
			// Try to read 1 byte from the request body. If it fails with the same error
			// it means the timeout occurred while reading the request body, so it's a 408.
			if _, readErr := r.Body.Read([]byte{0}); isNetworkTimeout(readErr) {
				statusCode = http.StatusRequestTimeout
				level.Error(logger).Log("msg", "response timeout", "err", err)
				break
			}
		}

		statusCode = http.StatusGatewayTimeout
		level.Error(logger).Log("msg", "network timeout", "err", err)
	default:
		level.Warn(logger).Log("msg", "request failed", "err", err)
		statusCode = http.StatusBadGateway
	}

	http.Error(w, err.Error(), statusCode)
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
