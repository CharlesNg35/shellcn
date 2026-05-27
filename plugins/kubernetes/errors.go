package kubernetes

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/charlesng35/shellcn/internal/plugin"
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
	case apierrors.IsServiceUnavailable(err), apierrors.IsServerTimeout(err), apierrors.IsTimeout(err):
		return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	default:
		return err
	}
}
