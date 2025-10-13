package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError provides a structured error that can be rendered to API consumers.
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Internal   error  `json:"-"`
}

func (e *AppError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}

	return e.Message
}

// Unwrap exposes the internal error for errors.Is / errors.As compatibility.
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Internal
}

// WithInternal returns a copy of the AppError with an attached internal error.
func (e *AppError) WithInternal(err error) *AppError {
	if e == nil {
		return nil
	}

	cpy := *e
	cpy.Internal = err
	return &cpy
}

// Common errors exposed to the rest of the application.
var (
	ErrUnauthorized = &AppError{
		Code:       "UNAUTHORIZED",
		Message:    "Authentication required",
		StatusCode: http.StatusUnauthorized,
	}
	ErrMFARequired = &AppError{
		Code:       "auth.mfa_required",
		Message:    "Multi-factor authentication required",
		StatusCode: http.StatusUnauthorized,
	}
	ErrMFAInvalid = &AppError{
		Code:       "auth.mfa_invalid",
		Message:    "Invalid multi-factor authentication code",
		StatusCode: http.StatusUnauthorized,
	}

	ErrInvalidCredentials = &AppError{
		Code:       "INVALID_CREDENTIALS",
		Message:    "Invalid username or password",
		StatusCode: http.StatusUnauthorized,
	}

	ErrForbidden = &AppError{
		Code:       "FORBIDDEN",
		Message:    "Permission denied",
		StatusCode: http.StatusForbidden,
	}

	ErrNotFound = &AppError{
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		StatusCode: http.StatusNotFound,
	}

	ErrBadRequest = &AppError{
		Code:       "BAD_REQUEST",
		Message:    "Invalid request",
		StatusCode: http.StatusBadRequest,
	}

	ErrInternalServer = &AppError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    "Internal server error",
		StatusCode: http.StatusInternalServerError,
	}

	ErrRateLimit = &AppError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "Too many requests, please slow down",
		StatusCode: http.StatusTooManyRequests,
	}

	ErrCSRFInvalid = &AppError{
		Code:       "CSRF_TOKEN_INVALID",
		Message:    "Invalid CSRF token",
		StatusCode: http.StatusForbidden,
	}
)

// New builds a new application error with the provided metadata.
func New(code, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Wrap turns any error into an AppError while keeping the original error for logging.
func Wrap(err error, message string) *AppError {
	return &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Internal:   err,
	}
}

// FromError converts a generic error into an AppError, defaulting to ErrInternalServer.
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return ErrInternalServer.WithInternal(err)
}

// NewBadRequest wraps validation errors with a helpful message.
func NewBadRequest(message string) *AppError {
	return &AppError{
		Code:       ErrBadRequest.Code,
		Message:    message,
		StatusCode: ErrBadRequest.StatusCode,
	}
}
