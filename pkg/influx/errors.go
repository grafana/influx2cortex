package influx

import (
	"net"

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
