package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type errorEnvelope struct {
	Error string `json:"error"`
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
	switch {
	case errors.Is(err, plugin.ErrInvalidInput), errors.Is(err, models.ErrInvalidInput):
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
	case errors.Is(err, plugin.ErrUnavailable), errors.Is(err, session.ErrSessionLimit),
		errors.Is(err, session.ErrChannelLimit), errors.Is(err, transport.ErrAgentUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, plugin.ErrNotSupported):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

// cleanAgentError replaces a noisy tunnel dial failure (internal URL +
// "session shutdown") with a clear, client-safe message.
func cleanAgentError(err error) error {
	if errors.Is(err, transport.ErrAgentUnavailable) {
		return fmt.Errorf("%w: the agent for this connection is offline", plugin.ErrUnavailable)
	}
	return err
}

// writeError normalizes any error to a JSON envelope + status. Genuine 5xx
// detail is logged and hidden; 503 (unavailable) carries its message through, as
// it is an expected, client-actionable state rather than a server fault.
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	err = cleanAgentError(err)
	status := statusFor(err)
	msg := err.Error()
	if status >= 500 && status != http.StatusServiceUnavailable {
		if log != nil {
			log.Error("request failed", "err", err)
		}
		msg = http.StatusText(status)
	}
	writeJSON(w, status, errorEnvelope{Error: msg})
}

func writeAuthRequired(w http.ResponseWriter, log *slog.Logger, err error) {
	w.Header().Set("X-ShellCN-Auth", "required")
	writeError(w, log, err)
}
