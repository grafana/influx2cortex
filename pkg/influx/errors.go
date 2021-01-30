package influx

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
)

type status string

const (
	StatusClientClosedRequest        = 499
	statusError               status = "error"
)

type errorType string

const (
	errorTimeout     errorType = "timeout"
	errorCanceled    errorType = "canceled"
	errorUnavailable errorType = "unavailable"
)

type ErrorResponse struct {
	Status    status    `json:"status"`
	ErrorType errorType `json:"errorType,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	response := ErrorResponse{
		Status: statusError,
		Error:  err.Error(),
	}
	var statusCode int
	switch {
	case errors.Is(err, context.Canceled):
		statusCode = StatusClientClosedRequest
		response.ErrorType = errorCanceled
	case errors.Is(err, context.DeadlineExceeded) || isGRPCTimeout(err):
		statusCode = http.StatusGatewayTimeout
		response.ErrorType = errorTimeout
	case isNetworkTimeout(err):
		if r.Body != nil {
			// Try to read 1 byte from the request body. If it fails with the same error
			// it means the timeout occurred while reading the request body, so it's a 408.
			if _, readErr := r.Body.Read([]byte{0}); isNetworkTimeout(readErr) {
				statusCode = http.StatusRequestTimeout
				response.ErrorType = errorTimeout
				break
			}
		}

		statusCode = http.StatusGatewayTimeout
		response.ErrorType = errorTimeout
	default:
		log.Warnf("Request failed: %v", err)
		statusCode = http.StatusBadGateway
		response.ErrorType = errorUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithError(err).Error("failed to encode error response")
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
	s, ok := grpc_status.FromError(errors.Cause(err))
	return ok && s.Code() == codes.DeadlineExceeded
}
