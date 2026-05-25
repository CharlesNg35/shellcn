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
