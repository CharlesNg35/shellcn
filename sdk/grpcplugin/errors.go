package grpcplugin

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

var codeToSentinel = map[codes.Code]error{
	codes.InvalidArgument:  plugin.ErrInvalidInput,
	codes.NotFound:         plugin.ErrNotFound,
	codes.Unauthenticated:  plugin.ErrUnauthorized,
	codes.PermissionDenied: plugin.ErrForbidden,
	codes.AlreadyExists:    plugin.ErrConflict,
	codes.Unavailable:      plugin.ErrUnavailable,
	codes.Unimplemented:    plugin.ErrNotSupported,
}

// ErrorFromStatus maps a gRPC error back to the matching plugin sentinel so the
// host's error handling behaves as it does for a built-in plugin.
func ErrorFromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	if sentinel, ok := codeToSentinel[st.Code()]; ok {
		return fmt.Errorf("%w: %s", sentinel, st.Message())
	}
	return errors.New(st.Message())
}

// StatusFromError maps a plugin sentinel to a gRPC status for the serve side.
func StatusFromError(err error) error {
	if err == nil {
		return nil
	}
	for code, sentinel := range codeToSentinel {
		if errors.Is(err, sentinel) {
			return status.Error(code, err.Error())
		}
	}
	return status.Error(codes.Unknown, err.Error())
}
