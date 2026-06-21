package kubernetes

import (
	stderrors "errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// apiErr maps a Kubernetes API error to a core sentinel so the server boundary
// renders the right HTTP status, preserving the API server's message.
func apiErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case apierrors.IsNotFound(err):
		return fmt.Errorf("%w: %v", plugin.ErrNotFound, err)
	case apierrors.IsAlreadyExists(err):
		return fmt.Errorf("%w: %v", plugin.ErrAlreadyExists, err)
	case apierrors.IsConflict(err):
		return fmt.Errorf("%w: %v", plugin.ErrConflict, err)
	case apierrors.IsUnauthorized(err):
		return fmt.Errorf("%w: %v", plugin.ErrUnauthorized, err)
	case apierrors.IsForbidden(err):
		return fmt.Errorf("%w: %v", plugin.ErrForbidden, err)
	case apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
	case apierrors.IsMethodNotSupported(err), apierrors.IsNotAcceptable(err), apierrors.IsUnsupportedMediaType(err):
		return fmt.Errorf("%w: %v", plugin.ErrNotSupported, err)
	case apierrors.IsServiceUnavailable(err), apierrors.IsServerTimeout(err), apierrors.IsTimeout(err):
		return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	}
	// Any other client-side (4xx) apiserver error surfaces its message as invalid
	// input rather than collapsing to an opaque 500 with the detail hidden.
	var status apierrors.APIStatus
	if stderrors.As(err, &status) {
		if code := status.Status().Code; code >= 400 && code < 500 {
			return fmt.Errorf("%w: %v", plugin.ErrInvalidInput, err)
		}
	}
	return err
}
