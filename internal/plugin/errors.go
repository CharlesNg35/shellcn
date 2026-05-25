package plugin

import "errors"

// Sentinel errors a handler or the core may return; the server boundary
// normalizes these to HTTP status codes.
var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotFound      = errors.New("not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrUnavailable   = errors.New("unavailable")
	ErrNotSupported  = errors.New("not supported")
	ErrAlreadyExists = errors.New("already exists")
)

// VerificationRequired is returned by Connect when a human decision is needed
// before proceeding (unknown SSH host key, untrusted TLS cert, MFA prompt). The
// core surfaces the structured payload to a generic confirm/decision panel.
type VerificationRequired struct {
	Kind    string         // e.g. "host_key", "tls_cert", "mfa"
	Message string         // human-readable prompt
	Data    map[string]any // fingerprint, old/new key, etc.
}

func (e *VerificationRequired) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "verification required: " + e.Kind
}
