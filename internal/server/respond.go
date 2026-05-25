package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
	"github.com/charlesng/shellcn/internal/session"
	"github.com/charlesng/shellcn/internal/store"
)

type errorEnvelope struct {
	Error string `json:"error"`
}

type verificationEnvelope struct {
	Error   string         `json:"error"`
	Kind    string         `json:"kind"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// statusFor maps a sentinel error to an HTTP status (the boundary normalization).
func statusFor(err error) int {
	var verify *plugin.VerificationRequired
	switch {
	case errors.As(err, &verify):
		return http.StatusConflict
	case errors.Is(err, plugin.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, plugin.ErrUnauthorized), errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, plugin.ErrForbidden), errors.Is(err, policy.ErrForbidden),
		errors.Is(err, models.ErrForbidden), errors.Is(err, auth.ErrAccountDisabled):
		return http.StatusForbidden
	case errors.Is(err, plugin.ErrNotFound), errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, plugin.ErrConflict), errors.Is(err, models.ErrConflict), errors.Is(err, plugin.ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, plugin.ErrUnavailable), errors.Is(err, session.ErrSessionLimit), errors.Is(err, session.ErrChannelLimit):
		return http.StatusServiceUnavailable
	case errors.Is(err, plugin.ErrNotSupported):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

// writeError normalizes any error to a JSON envelope + status. Server (5xx)
// errors are logged; their detail is not leaked to the client.
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	status := statusFor(err)
	var verify *plugin.VerificationRequired
	if errors.As(err, &verify) {
		writeJSON(w, status, verificationEnvelope{
			Error:   "verification required",
			Kind:    verify.Kind,
			Message: verify.Message,
			Data:    verify.Data,
		})
		return
	}
	msg := err.Error()
	if status >= 500 {
		if log != nil {
			log.Error("request failed", "err", err)
		}
		msg = http.StatusText(status)
	}
	writeJSON(w, status, errorEnvelope{Error: msg})
}
