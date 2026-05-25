package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/service"
)

const (
	connCreateEvent = "connection.create"
	connUpdateEvent = "connection.update"
	connDeleteEvent = "connection.delete"
)

type connectionWriteRequest struct {
	Name      string            `json:"name"`
	Protocol  string            `json:"protocol"`
	Transport string            `json:"transport"`
	Config    map[string]any    `json:"config"`
	Recording map[string]string `json:"recording"`
}

func (s *Server) toConnectionDTO(c models.Connection) connectionDTO {
	dto := connectionDTO{
		ID: c.ID, Name: c.Name, Protocol: c.Protocol,
		Transport: c.Transport, Online: c.Transport != string(plugin.TransportAgent),
		Recording: c.Recording,
	}
	if dto.Transport == string(plugin.TransportAgent) {
		dto.Status = "pending"
	}
	if m, ok := s.deps.Plugins.Manifest(c.Protocol); ok {
		icon := m.Icon
		dto.Icon = &icon
	}
	return dto
}

func (s *Server) auditConnEvent(ctx context.Context, user models.User, connID, event string, risk plugin.RiskLevel, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, ConnectionID: connID, RouteID: event,
		Risk: string(risk), Result: result, Err: err,
	})
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)

	var req connectionWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	conn, err := s.deps.Connections.Create(ctx, user.ID, service.ConnectionInput{
		Name: req.Name, Protocol: req.Protocol, Transport: req.Transport,
		Config: req.Config, Recording: req.Recording,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, "", connCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, conn.ID, connCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	dto := s.toConnectionDTO(conn)
	dto.CanManage = true
	writeJSON(w, http.StatusCreated, dto)
}

func (s *Server) handleConnectionDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canManageConnection(ctx, user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	writeJSON(w, http.StatusOK, s.deps.Connections.Detail(conn))
}

func (s *Server) handleUpdateConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canManageConnection(ctx, user, conn) {
		s.auditConnEvent(ctx, user, conn.ID, connUpdateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	var req connectionWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	updated, err := s.deps.Connections.Update(ctx, conn, service.ConnectionInput{
		Name: req.Name, Transport: req.Transport, Config: req.Config, Recording: req.Recording,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, conn.ID, connUpdateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, conn.ID, connUpdateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, s.deps.Connections.Detail(updated))
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canManageConnection(ctx, user, conn) {
		s.auditConnEvent(ctx, user, conn.ID, connDeleteEvent, plugin.RiskDestructive, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	if err := s.deps.Connections.Delete(ctx, conn.ID); err != nil {
		s.auditConnEvent(ctx, user, conn.ID, connDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.cleanupConnectionDependents(ctx, conn.ID)
	s.auditConnEvent(ctx, user, conn.ID, connDeleteEvent, plugin.RiskDestructive, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// cleanupConnectionDependents removes the access-control state tied to a deleted
// connection so it can never be inherited by a future record: it drops any live
// agent tunnel, deletes sharing grants, and revokes outstanding enrollments.
// Best-effort — the connection is already gone, so failures are logged not fatal.
func (s *Server) cleanupConnectionDependents(ctx context.Context, connID string) {
	if s.deps.Tunnels != nil {
		s.deps.Tunnels.Remove(connID)
	}
	if grants, err := s.deps.Store.Grants.ListByConnection(ctx, connID); err == nil {
		for _, g := range grants {
			if err := s.deps.Store.Grants.Delete(ctx, g.ID); err != nil {
				s.deps.Logger.Warn("cleanup grant failed", "connection", connID, "grant", g.ID, "err", err)
			}
		}
	}
	if enrs, err := s.deps.Store.Enrollments.ListByConnection(ctx, connID); err == nil {
		for _, e := range enrs {
			if e.Status == models.EnrollmentPending || e.Status == models.EnrollmentOnline {
				if err := s.deps.Store.Enrollments.UpdateStatus(ctx, e.ID, models.EnrollmentRevoked); err != nil {
					s.deps.Logger.Warn("cleanup enrollment failed", "connection", connID, "enrollment", e.ID, "err", err)
				}
			}
		}
	}
}
