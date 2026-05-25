package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userDTO struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	DisplayName string        `json:"displayName,omitempty"`
	Email       string        `json:"email,omitempty"`
	Roles       []models.Role `json:"roles"`
	Protected   bool          `json:"protected"`
}

type sessionDTO struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrfToken"`
}

func toUserDTO(u models.User) userDTO {
	roles := u.Roles
	if roles == nil {
		roles = []models.Role{}
	}
	return userDTO{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Roles: roles, Protected: u.Protected}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	user, err := s.deps.Auth.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		// Collapse "account disabled" into the generic invalid-credentials
		// response: a distinct 403 would let an attacker enumerate accounts and
		// confirm a correct password against a disabled account.
		if errors.Is(err, auth.ErrAccountDisabled) {
			err = auth.ErrInvalidCredentials
		}
		writeError(w, s.deps.Logger, err)
		return
	}
	sess := s.deps.SessionMgr.Create(user.ID)
	auth.SetSessionCookie(w, sess, isTLS(r))
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(user), CSRFToken: sess.CSRFToken})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := sessionFrom(r.Context()); ok {
		s.deps.SessionMgr.Destroy(sess.ID)
	}
	auth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	sess, _ := sessionFrom(r.Context())
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(user), CSRFToken: sess.CSRFToken})
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
	if len(req.NewPassword) < 8 {
		writeError(w, s.deps.Logger, fmt.Errorf("%w: password must be at least 8 characters", plugin.ErrInvalidInput))
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
	s.auditAccountEvent(ctx, user, "account.password.change", models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
