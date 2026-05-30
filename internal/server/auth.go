package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

const (
	loginEvent  = "auth.login"
	logoutEvent = "auth.logout"
	mfaEvent    = "auth.mfa"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type mfaRequest struct {
	MFAToken string `json:"mfaToken"`
	Code     string `json:"code"`
}

type userDTO struct {
	ID               string        `json:"id"`
	Username         string        `json:"username"`
	DisplayName      string        `json:"displayName,omitempty"`
	Email            string        `json:"email,omitempty"`
	Roles            []models.Role `json:"roles"`
	Protected        bool          `json:"protected"`
	TwoFactorEnabled bool          `json:"twoFactorEnabled"`
}

type sessionDTO struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrfToken"`
	// MFAReminder asks the client to nudge the user to enable 2FA after sign-in.
	MFAReminder bool `json:"mfaReminder"`
}

// loginResponse is either an MFA challenge (password verified, second factor
// pending) or a completed session — never both.
type loginResponse struct {
	MFARequired bool        `json:"mfaRequired"`
	MFAToken    string      `json:"mfaToken,omitempty"`
	Session     *sessionDTO `json:"session,omitempty"`
}

func toUserDTO(u models.User) userDTO {
	roles := u.Roles
	if roles == nil {
		roles = []models.Role{}
	}
	return userDTO{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Email: u.Email,
		Roles: roles, Protected: u.Protected, TwoFactorEnabled: u.TOTPEnabled,
	}
}

func (s *Server) auditAuth(ctx context.Context, user models.User, event string, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, RouteID: event, Risk: string(plugin.RiskSafe), Result: result, Err: err,
	})
}

func (s *Server) shouldRemindMFA(user models.User) bool {
	return s.deps.TwoFactor != nil && s.deps.TwoFactor.ShouldRemind(user)
}

func (s *Server) sessionDTOFor(user models.User, csrf string) *sessionDTO {
	return &sessionDTO{User: toUserDTO(user), CSRFToken: csrf, MFAReminder: s.shouldRemindMFA(user)}
}

// startSession mints a session cookie for an already-authenticated user.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user models.User) *sessionDTO {
	sess := s.deps.SessionMgr.Create(user.ID, user.SessionVersion)
	auth.SetSessionCookie(w, sess, isTLS(r))
	return s.sessionDTOFor(user, sess.CSRFToken)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	user, err := s.deps.Auth.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		// Collapse "account disabled" into the generic invalid-credentials
		// response so an attacker can't confirm a correct password against a
		// disabled account.
		if errors.Is(err, auth.ErrAccountDisabled) {
			err = auth.ErrInvalidCredentials
		}
		s.auditAuth(ctx, models.User{Username: req.Username}, loginEvent, models.AuditDenied, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	// 2FA-enabled accounts get a challenge, not a session — the session is only
	// minted once the second factor is verified.
	if user.TOTPEnabled && s.deps.TwoFactor != nil {
		token, _ := s.deps.SessionMgr.CreateMFAChallenge(user.ID)
		writeJSON(w, http.StatusOK, loginResponse{MFARequired: true, MFAToken: token})
		return
	}
	session := s.startSession(w, r, user)
	s.auditAuth(ctx, user, loginEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, loginResponse{Session: session})
}

// handleLoginMFA completes a two-step login: it verifies the challenge token and
// the second-factor code, then mints the session.
func (s *Server) handleLoginMFA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if s.deps.TwoFactor == nil {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	var req mfaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	userID, ok := s.deps.SessionMgr.ParseMFAChallenge(req.MFAToken)
	if !ok {
		s.auditAuth(ctx, models.User{}, mfaEvent, models.AuditDenied, auth.ErrInvalidCredentials)
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}
	user, err := s.deps.Store.Users.GetByID(ctx, userID)
	if err != nil || user.Disabled {
		s.auditAuth(ctx, models.User{ID: userID}, mfaEvent, models.AuditDenied, auth.ErrInvalidCredentials)
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}
	valid, err := s.deps.TwoFactor.Verify(ctx, user, req.Code)
	if err != nil || !valid {
		s.auditAuth(ctx, user, mfaEvent, models.AuditDenied, auth.ErrInvalidCredentials)
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}
	session := s.startSession(w, r, user)
	s.auditAuth(ctx, user, loginEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, loginResponse{Session: session})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	if sess, ok := sessionFrom(ctx); ok {
		s.deps.SessionMgr.Destroy(sess.ID)
	}
	auth.ClearSessionCookie(w)
	s.auditAuth(ctx, user, logoutEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	sess, _ := sessionFrom(r.Context())
	writeJSON(w, http.StatusOK, s.sessionDTOFor(user, sess.CSRFToken))
}

func (s *Server) auditAccountEvent(ctx context.Context, user models.User, event string, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, RouteID: event, Risk: string(plugin.RiskWrite), Result: result, Err: err,
	})
}

type updateProfileRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// handleUpdateProfile lets the signed-in user edit their own display name and
// email. Username, roles, and enabled state are not editable here.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	updated, err := s.deps.Users.UpdateProfile(ctx, user.ID, strings.TrimSpace(req.Email), strings.TrimSpace(req.DisplayName))
	if err != nil {
		s.auditAccountEvent(ctx, user, "account.profile.update", models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAccountEvent(ctx, user, "account.profile.update", models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, toUserDTO(updated))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// handleChangePassword changes the signed-in user's own password after verifying
// the current one.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	err := s.deps.Users.ChangePassword(ctx, user.ID, req.CurrentPassword, req.NewPassword)
	if errors.Is(err, service.ErrWrongPassword) {
		s.auditAccountEvent(ctx, user, "account.password.change", models.AuditDenied, err)
		writeError(w, s.deps.Logger, fmt.Errorf("%w: current password is incorrect", plugin.ErrInvalidInput))
		return
	}
	if err != nil {
		s.auditAccountEvent(ctx, user, "account.password.change", models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	updated, err := s.deps.Users.Get(ctx, user.ID)
	if err != nil {
		s.auditAccountEvent(ctx, user, "account.password.change", models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	sess := s.deps.SessionMgr.Create(updated.ID, updated.SessionVersion)
	auth.SetSessionCookie(w, sess, isTLS(r))
	s.auditAccountEvent(ctx, user, "account.password.change", models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(updated), CSRFToken: sess.CSRFToken})
}
