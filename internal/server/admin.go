package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

const (
	userCreateEvent   = "user.create"
	userUpdateEvent   = "user.update"
	userDeleteEvent   = "user.delete"
	inviteCreateEvent = "invitation.create"
	inviteRevokeEvent = "invitation.revoke"
)

type adminUserDTO struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	Email       string        `json:"email,omitempty"`
	DisplayName string        `json:"displayName,omitempty"`
	Roles       []models.Role `json:"roles"`
	Disabled    bool          `json:"disabled"`
	Protected   bool          `json:"protected"`
}

func toAdminUserDTO(u models.User) adminUserDTO {
	roles := u.Roles
	if roles == nil {
		roles = []models.Role{}
	}
	return adminUserDTO{
		ID: u.ID, Username: u.Username, Email: u.Email, DisplayName: u.DisplayName,
		Roles: roles, Disabled: u.Disabled, Protected: u.Protected,
	}
}

func validRole(s string) (models.Role, bool) {
	switch models.Role(s) {
	case models.RoleAdmin, models.RoleOperator, models.RoleViewer:
		return models.Role(s), true
	}
	return "", false
}

func (s *Server) auditAdminEvent(ctx context.Context, user models.User, event string, result models.AuditResult, params map[string]string, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, RouteID: event, Risk: string(plugin.RiskPrivileged),
		Result: result, Params: params, Err: err,
	})
}

// --- users ------------------------------------------------------------------

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Users.List(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]adminUserDTO, 0, len(list))
	for _, u := range list {
		out = append(out, toAdminUserDTO(u))
	}
	writeJSON(w, http.StatusOK, out)
}

type createUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	role, ok := validRole(req.Role)
	if !ok || strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	user, err := s.deps.Users.Create(ctx, service.NewUserInput{
		Username: req.Username, Email: req.Email, DisplayName: req.DisplayName,
		Roles: []models.Role{role}, Password: req.Password,
	})
	if err != nil {
		s.auditAdminEvent(ctx, actor, userCreateEvent, models.AuditError, nil, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, userCreateEvent, models.AuditAllowed, map[string]string{"username": user.Username}, nil)
	writeJSON(w, http.StatusCreated, toAdminUserDTO(user))
}

type updateUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)
	target, err := s.deps.Users.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	role, ok := validRole(req.Role)
	if !ok {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	// The root admin must stay an enabled admin (no self-lockout).
	if target.Protected && (req.Disabled || role != models.RoleAdmin) {
		s.auditAdminEvent(ctx, actor, userUpdateEvent, models.AuditDenied, map[string]string{"username": target.Username}, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, errForbidden("the root admin must remain an enabled admin"))
		return
	}
	if target.HasRole(models.RoleAdmin) && !actor.Protected && (req.Disabled != target.Disabled || role != models.RoleAdmin) {
		s.auditAdminEvent(ctx, actor, userUpdateEvent, models.AuditDenied, map[string]string{"username": target.Username}, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, errForbidden("only the root admin may disable or change another admin's role"))
		return
	}
	updated, err := s.deps.Users.Update(ctx, target.ID, service.UpdateUserInput{
		Email: req.Email, DisplayName: req.DisplayName, Roles: []models.Role{role}, Disabled: req.Disabled,
	})
	if err != nil {
		s.auditAdminEvent(ctx, actor, userUpdateEvent, models.AuditError, nil, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, userUpdateEvent, models.AuditAllowed, map[string]string{"username": updated.Username}, nil)
	writeJSON(w, http.StatusOK, toAdminUserDTO(updated))
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)
	target, err := s.deps.Users.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if target.Protected {
		s.auditAdminEvent(ctx, actor, userDeleteEvent, models.AuditDenied, nil, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, errForbidden("the root admin cannot be deleted"))
		return
	}
	// Only the root admin may delete other admins.
	if target.HasRole(models.RoleAdmin) && !actor.Protected {
		s.auditAdminEvent(ctx, actor, userDeleteEvent, models.AuditDenied, nil, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, errForbidden("only the root admin may delete an admin"))
		return
	}
	if err := s.deps.Users.Delete(ctx, target.ID); err != nil {
		s.auditAdminEvent(ctx, actor, userDeleteEvent, models.AuditError, nil, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, userDeleteEvent, models.AuditAllowed, map[string]string{"username": target.Username}, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- invitations (admin) ----------------------------------------------------

func (s *Server) handleAdminListInvitations(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Invitations.List(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type createInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteResponse struct {
	Invitation models.InvitationSummary `json:"invitation"`
	Link       string                   `json:"link"`
	EmailSent  bool                     `json:"emailSent"`
}

func (s *Server) handleAdminCreateInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)

	var req createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	role, ok := validRole(req.Role)
	if !ok || !strings.Contains(req.Email, "@") {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	inv, token, emailSent, err := s.deps.Invitations.Create(ctx, req.Email, role, actor.ID, s.inviteAcceptURL(r))
	if err != nil {
		s.auditAdminEvent(ctx, actor, inviteCreateEvent, models.AuditError, nil, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, inviteCreateEvent, models.AuditAllowed, map[string]string{"email": req.Email}, nil)
	writeJSON(w, http.StatusCreated, inviteResponse{
		Invitation: inv.Summary(),
		Link:       s.inviteAcceptURL(r) + token,
		EmailSent:  emailSent,
	})
}

func (s *Server) handleAdminRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)
	id := chi.URLParam(r, "id")
	if err := s.deps.Invitations.Revoke(ctx, id); err != nil {
		s.auditAdminEvent(ctx, actor, inviteRevokeEvent, models.AuditError, map[string]string{"id": id}, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, inviteRevokeEvent, models.AuditAllowed, map[string]string{"id": id}, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAdminEmailStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": s.deps.Invitations.EmailEnabled()})
}

// --- invitation accept (public) ---------------------------------------------

func (s *Server) handleInvitationLookup(w http.ResponseWriter, r *http.Request) {
	inv, err := s.deps.Invitations.Lookup(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"email": inv.Email, "role": string(inv.Role)})
}

type acceptInviteRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	user, err := s.deps.Invitations.Accept(ctx, chi.URLParam(r, "token"), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvitationInvalid) {
			writeError(w, s.deps.Logger, plugin.ErrNotFound)
			return
		}
		writeError(w, s.deps.Logger, err) // models.ErrConflict → 409 on a taken username
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"username": user.Username})
}

func (s *Server) inviteAcceptURL(r *http.Request) string {
	scheme := "http"
	if isTLS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/invite/"
}

func errForbidden(msg string) error {
	return &wrappedForbidden{msg: msg}
}

type wrappedForbidden struct{ msg string }

func (e *wrappedForbidden) Error() string { return e.msg }
func (e *wrappedForbidden) Unwrap() error { return plugin.ErrForbidden }
