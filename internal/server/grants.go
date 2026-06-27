package server

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	connGrantCreateEvent = "connection.grant.create"
	connGrantDeleteEvent = "connection.grant.delete"
	credGrantCreateEvent = "credential.grant.create"
	credGrantDeleteEvent = "credential.grant.delete"
)

type grantRequest struct {
	// Admins pick a subject by id (autocomplete); operators, who cannot enumerate
	// users, share by exact email — exactly one is required.
	SubjectID string `json:"subjectId"`
	Email     string `json:"email"`
	Access    string `json:"access"`
}

// resolveGrantSubject maps a grant request to a target user id: a picked id, or
// an exact-email lookup.
func (s *Server) resolveGrantSubject(ctx context.Context, req grantRequest) (string, error) {
	if req.SubjectID != "" {
		if _, err := s.deps.Store.Users.GetByID(ctx, req.SubjectID); err != nil {
			return "", plugin.ErrInvalidInput
		}
		return req.SubjectID, nil
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return "", plugin.ErrInvalidInput
	}
	u, err := s.deps.Store.Users.GetByEmail(ctx, email)
	if err != nil {
		return "", plugin.ErrNotFound
	}
	return u.ID, nil
}

type grantDTO struct {
	ID          string `json:"id"`
	SubjectID   string `json:"subjectId"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Access      string `json:"access"`
}

// isOwner gates sharing (grant create/list/revoke): only the resource owner may
// share. Admin is a user-management role and confers no access to others' shares.
func isOwner(user models.User, ownerID string) bool {
	return ownerID == user.ID
}

// subjectLabel resolves a subject id to its username/display name for display.
func (s *Server) subjectLabel(ctx context.Context, subjectID string) (string, string) {
	u, err := s.deps.Store.Users.GetByID(ctx, subjectID)
	if err != nil {
		return "", ""
	}
	return u.Username, u.DisplayName
}

// --- connection grants ------------------------------------------------------

func (s *Server) handleListConnectionGrants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, conn.OwnerID) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	grants, err := s.deps.Store.Grants.ListByConnection(ctx, conn.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]grantDTO, 0, len(grants))
	for _, g := range grants {
		username, display := s.subjectLabel(ctx, g.SubjectID)
		out = append(out, grantDTO{ID: g.ID, SubjectID: g.SubjectID, Username: username, DisplayName: display, Access: string(g.Access)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateConnectionGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, conn.OwnerID) {
		s.auditConnEvent(ctx, user, conn.ID, connGrantCreateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	req, access, ok := s.decodeGrant(w, r, models.ConnectionGrantAccesses()...)
	if !ok {
		return
	}
	subjectID, err := s.resolveGrantSubject(ctx, req)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	g := models.Grant{ID: uuid.NewString(), ConnectionID: conn.ID, SubjectID: subjectID, Access: access}
	if err := s.deps.Store.Grants.Create(ctx, &g); err != nil {
		s.auditConnEvent(ctx, user, conn.ID, connGrantCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, conn.ID, connGrantCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	username, display := s.subjectLabel(ctx, g.SubjectID)
	writeJSON(w, http.StatusCreated, grantDTO{ID: g.ID, SubjectID: g.SubjectID, Username: username, DisplayName: display, Access: string(g.Access)})
}

func (s *Server) handleDeleteConnectionGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, conn.OwnerID) {
		s.auditConnEvent(ctx, user, conn.ID, connGrantDeleteEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	grantID := chi.URLParam(r, "grantId")
	if !connectionGrantBelongs(ctx, s.deps.Store.Grants, conn.ID, grantID) {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	if err := s.deps.Store.Grants.Delete(ctx, grantID); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, conn.ID, connGrantDeleteEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- credential grants ------------------------------------------------------

func (s *Server) handleListCredentialGrants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	cred, err := s.deps.Store.Credentials.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, cred.OwnerID) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	grants, err := s.deps.Store.CredentialGrants.ListByCredential(ctx, cred.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]grantDTO, 0, len(grants))
	for _, g := range grants {
		username, display := s.subjectLabel(ctx, g.SubjectID)
		out = append(out, grantDTO{ID: g.ID, SubjectID: g.SubjectID, Username: username, DisplayName: display, Access: string(g.Access)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateCredentialGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	cred, err := s.deps.Store.Credentials.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, cred.OwnerID) {
		s.auditCredEvent(ctx, user, cred.ID, credGrantCreateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	// Credentials confer view only — they never grant secret readback.
	req, access, ok := s.decodeGrant(w, r, models.CredentialGrantAccesses()...)
	if !ok {
		return
	}
	subjectID, err := s.resolveGrantSubject(ctx, req)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	g := models.CredentialGrant{ID: uuid.NewString(), CredentialID: cred.ID, SubjectID: subjectID, Access: access}
	if err := s.deps.Store.CredentialGrants.Create(ctx, &g); err != nil {
		s.auditCredEvent(ctx, user, cred.ID, credGrantCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditCredEvent(ctx, user, cred.ID, credGrantCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	username, display := s.subjectLabel(ctx, g.SubjectID)
	writeJSON(w, http.StatusCreated, grantDTO{ID: g.ID, SubjectID: g.SubjectID, Username: username, DisplayName: display, Access: string(g.Access)})
}

func (s *Server) handleDeleteCredentialGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	cred, err := s.deps.Store.Credentials.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !isOwner(user, cred.OwnerID) {
		s.auditCredEvent(ctx, user, cred.ID, credGrantDeleteEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	grantID := chi.URLParam(r, "grantId")
	if !credentialGrantBelongs(ctx, s.deps.Store.CredentialGrants, cred.ID, grantID) {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	if err := s.deps.Store.CredentialGrants.Delete(ctx, grantID); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditCredEvent(ctx, user, cred.ID, credGrantDeleteEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func connectionGrantBelongs(ctx context.Context, grants storeGrantLister, connectionID, grantID string) bool {
	list, err := grants.ListByConnection(ctx, connectionID)
	if err != nil {
		return false
	}
	for _, g := range list {
		if g.ID == grantID {
			return true
		}
	}
	return false
}

func credentialGrantBelongs(ctx context.Context, grants storeCredentialGrantLister, credentialID, grantID string) bool {
	list, err := grants.ListByCredential(ctx, credentialID)
	if err != nil {
		return false
	}
	for _, g := range list {
		if g.ID == grantID {
			return true
		}
	}
	return false
}

type storeGrantLister interface {
	ListByConnection(context.Context, string) ([]models.Grant, error)
}

type storeCredentialGrantLister interface {
	ListByCredential(context.Context, string) ([]models.CredentialGrant, error)
}

// decodeGrant decodes the body and validates the access against the allowed set.
func (s *Server) decodeGrant(w http.ResponseWriter, r *http.Request, allowed ...models.Access) (grantRequest, models.Access, bool) {
	var req grantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		(req.SubjectID == "" && strings.TrimSpace(req.Email) == "") {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return req, "", false
	}
	access := models.Access(req.Access)
	if slices.Contains(allowed, access) {
		return req, access, true
	}
	writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
	return req, "", false
}
