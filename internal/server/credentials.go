package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

const (
	credCreateEvent = "credential.create"
	credUpdateEvent = "credential.update"
	credDeleteEvent = "credential.delete"
)

type credentialWriteRequest struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Identity string `json:"identity"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

func (r credentialWriteRequest) principal() string {
	if strings.TrimSpace(r.Identity) != "" {
		return r.Identity
	}
	return r.Username
}

func canManageCredential(user models.User, cred models.Credential) bool {
	return cred.OwnerID == user.ID
}

func (s *Server) auditCredEvent(ctx context.Context, user models.User, credID, event string, risk plugin.RiskLevel, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, RouteID: event, Risk: string(risk), Result: result,
		Params: map[string]string{"credentialId": credID}, Err: err,
	})
}

func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	if !canCreate(user) {
		s.auditCredEvent(ctx, user, "", credCreateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	var req credentialWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	cred, err := s.deps.Credentials.Create(ctx, service.NewCredentialInput{
		OwnerID: user.ID, Name: req.Name, Kind: req.Kind,
		Identity: req.principal(), Secret: req.Secret,
	})
	if err != nil {
		s.auditCredEvent(ctx, user, "", credCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditCredEvent(ctx, user, cred.ID, credCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusCreated, cred.Summary())
}

func (s *Server) handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	cred, err := s.deps.Store.Credentials.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !canManageCredential(user, cred) {
		s.auditCredEvent(ctx, user, cred.ID, credUpdateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	var req credentialWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	updated, err := s.deps.Credentials.Update(ctx, cred.ID, service.UpdateCredentialInput{
		Name: req.Name, Kind: req.Kind, Identity: req.principal(),
		Secret: req.Secret,
	})
	if err != nil {
		s.auditCredEvent(ctx, user, cred.ID, credUpdateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditCredEvent(ctx, user, cred.ID, credUpdateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, updated.Summary())
}

func (s *Server) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	cred, err := s.deps.Store.Credentials.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !canManageCredential(user, cred) {
		s.auditCredEvent(ctx, user, cred.ID, credDeleteEvent, plugin.RiskDestructive, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	if s.deps.Connections != nil {
		referenced, err := s.deps.Connections.ReferencesCredential(ctx, cred.ID)
		if err != nil {
			writeError(w, s.deps.Logger, err)
			return
		}
		if referenced {
			err := models.ErrConflict
			s.auditCredEvent(ctx, user, cred.ID, credDeleteEvent, plugin.RiskDestructive, models.AuditDenied, err)
			writeError(w, s.deps.Logger, err)
			return
		}
	}
	if err := s.deps.Credentials.Delete(ctx, cred.ID); err != nil {
		s.auditCredEvent(ctx, user, cred.ID, credDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.cleanupCredentialDependents(ctx, cred.ID)
	s.auditCredEvent(ctx, user, cred.ID, credDeleteEvent, plugin.RiskDestructive, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// cleanupCredentialDependents deletes the use-grants tied to a deleted credential
// so they cannot be inherited by a future record. Best-effort.
func (s *Server) cleanupCredentialDependents(ctx context.Context, credID string) {
	grants, err := s.deps.Store.CredentialGrants.ListByCredential(ctx, credID)
	if err != nil {
		return
	}
	for _, g := range grants {
		if err := s.deps.Store.CredentialGrants.Delete(ctx, g.ID); err != nil {
			s.deps.Logger.Warn("cleanup credential grant failed", "credential", credID, "grant", g.ID, "err", err)
		}
	}
}
