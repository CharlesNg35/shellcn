package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

const (
	totpEnableEvent  = "account.2fa.enable"
	totpDisableEvent = "account.2fa.disable"
)

type totpSetupResponse struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauthUrl"`
	QR         string `json:"qr"`
}

type totpCodeRequest struct {
	Code string `json:"code"`
}

type recoveryCodesResponse struct {
	RecoveryCodes []string `json:"recoveryCodes"`
}

// mapTwoFactorErr turns service errors into client-facing HTTP errors without
// leaking which check failed beyond "the code was wrong".
func mapTwoFactorErr(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidCode):
		return fmt.Errorf("%w: invalid code", plugin.ErrInvalidInput)
	case errors.Is(err, service.ErrTOTPNotEnrolled), errors.Is(err, service.ErrTOTPNotEnabled):
		return fmt.Errorf("%w: two-factor authentication is not set up", plugin.ErrInvalidInput)
	case errors.Is(err, service.ErrTOTPAlreadyEnabled):
		return fmt.Errorf("%w: two-factor authentication is already enabled", plugin.ErrInvalidInput)
	default:
		return err
	}
}

func (s *Server) auditTwoFactor(ctx context.Context, user models.User, event string, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, RouteID: event, Risk: string(plugin.RiskPrivileged), Result: result, Err: err,
	})
}

func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	enrollment, err := s.deps.TwoFactor.BeginEnrollment(ctx, user)
	if err != nil {
		writeError(w, s.deps.Logger, mapTwoFactorErr(err))
		return
	}
	writeJSON(w, http.StatusOK, totpSetupResponse{
		Secret: enrollment.Secret, OTPAuthURL: enrollment.OTPAuthURL, QR: enrollment.QRDataURL,
	})
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	req, ok := decodeCode(w, s, r)
	if !ok {
		return
	}
	codes, err := s.deps.TwoFactor.ConfirmEnrollment(ctx, user, req.Code)
	if err != nil {
		s.auditTwoFactor(ctx, user, totpEnableEvent, models.AuditDenied, err)
		writeError(w, s.deps.Logger, mapTwoFactorErr(err))
		return
	}
	s.auditTwoFactor(ctx, user, totpEnableEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	req, ok := decodeCode(w, s, r)
	if !ok {
		return
	}
	if err := s.deps.TwoFactor.Disable(ctx, user, req.Code); err != nil {
		s.auditTwoFactor(ctx, user, totpDisableEvent, models.AuditDenied, err)
		writeError(w, s.deps.Logger, mapTwoFactorErr(err))
		return
	}
	s.auditTwoFactor(ctx, user, totpDisableEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTOTPRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	req, ok := decodeCode(w, s, r)
	if !ok {
		return
	}
	codes, err := s.deps.TwoFactor.RegenerateRecoveryCodes(ctx, user, req.Code)
	if err != nil {
		writeError(w, s.deps.Logger, mapTwoFactorErr(err))
		return
	}
	writeJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: codes})
}

// handleTOTPRemind records that the user dismissed the 2FA nudge, so the prompt
// waits out the reminder interval before showing again.
func (s *Server) handleTOTPRemind(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	if err := s.deps.TwoFactor.Remind(ctx, user.ID); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func decodeCode(w http.ResponseWriter, s *Server, r *http.Request) (totpCodeRequest, bool) {
	var req totpCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return req, false
	}
	return req, true
}
