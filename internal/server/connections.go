package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	connCreateEvent            = "connection.create"
	connUpdateEvent            = "connection.update"
	connDeleteEvent            = "connection.delete"
	connSessionDisconnectEvent = "connection.session.disconnect"
	connFolderCreateEvent      = "connection_folder.create"
	connFolderUpdateEvent      = "connection_folder.update"
	connFolderDeleteEvent      = "connection_folder.delete"
	connLayoutUpdateEvent      = "connection_layout.update"
)

// Surfaced on a connection to drive the sidebar dot. The "connected" (green)
// state is client-side; the server only reports an agent with no live tunnel.
const connStatusOffline = "offline"

type connectionWriteRequest struct {
	Name                string            `json:"name"`
	Protocol            string            `json:"protocol"`
	Transport           string            `json:"transport"`
	Config              map[string]any    `json:"config"`
	PreserveCredentials []string          `json:"preserveCredentials"`
	Recording           map[string]string `json:"recording"`
	AIMode              string            `json:"aiMode"`
	AIAllowDestructive  bool              `json:"aiAllowDestructive"`
}

type connectionSessionDTO struct {
	State           string `json:"state"`
	Reason          string `json:"reason,omitempty"`
	Channels        int    `json:"channels"`
	Streams         int    `json:"streams"`
	LastSeen        string `json:"lastSeen,omitempty"`
	LastHealthCheck string `json:"lastHealthCheck,omitempty"`
	IdleExpiresIn   int64  `json:"idleExpiresIn,omitempty"`
}

// toConnectionDTO projects a stored connection for the client.
func (s *Server) toConnectionDTO(c models.Connection) connectionDTO {
	dto := connectionDTO{
		ID: c.ID, Name: c.Name, Protocol: c.Protocol,
		Transport: c.Transport, Recording: c.Recording,
		AIMode: c.AIMode, AIAllowDestructive: c.AIAllowDestructive,
	}
	// A direct transport is always dialable on demand; an agent transport is
	// reachable only while its tunnel is registered. `online` gates the enroll
	// panel; an offline agent surfaces a red dot (the green "connected" state is
	// tracked client-side, since a pooled session has no protocol-agnostic mark).
	dto.Online = true
	if dto.Transport == string(plugin.TransportAgent) {
		dto.Online = s.tunnelRegistered(c.ID)
	}
	if !dto.Online {
		dto.Status = connStatusOffline
	}
	if m, ok := s.deps.Plugins.Manifest(c.Protocol); ok {
		icon := m.Icon
		dto.Icon = &icon
	}
	return dto
}

// tunnelRegistered reports whether an agent tunnel is currently live for a
// connection — the authoritative source of agent reachability.
func (s *Server) tunnelRegistered(connID string) bool {
	if s.deps.Tunnels == nil {
		return false
	}
	_, ok := s.deps.Tunnels.Dialer(connID)
	return ok
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
	if !canCreate(user) {
		s.auditConnEvent(ctx, user, "", connCreateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	var req connectionWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	conn, err := s.deps.Connections.Create(ctx, user.ID, service.ConnectionInput{
		Name: req.Name, Protocol: req.Protocol, Transport: req.Transport,
		Config: req.Config, ActorID: user.ID, Recording: req.Recording,
		AIMode: req.AIMode, AIAllowDestructive: req.AIAllowDestructive,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, "", connCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, conn.ID, connCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	dto := s.toConnectionDTO(conn)
	s.decorateConnectionAccess(ctx, user, conn, &dto, map[string]string{})
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
	writeJSON(w, http.StatusOK, s.deps.Connections.Detail(ctx, user.ID, conn))
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
		Name: req.Name, Transport: req.Transport, Config: req.Config,
		ActorID: user.ID, PreserveCredentials: req.PreserveCredentials,
		Recording: req.Recording,
		AIMode:    req.AIMode, AIAllowDestructive: req.AIAllowDestructive,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, conn.ID, connUpdateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.deps.Sessions.CloseConnection(conn.ID)
	s.auditConnEvent(ctx, user, conn.ID, connUpdateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, s.deps.Connections.Detail(ctx, user.ID, updated))
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
	if err := s.deps.Store.ConnectionPlacements.DeleteByConnection(ctx, conn.ID); err != nil {
		s.deps.Logger.Warn("cleanup connection placements failed", "connection", conn.ID, "err", err)
	}
	s.cleanupConnectionDependents(ctx, conn.ID)
	s.auditConnEvent(ctx, user, conn.ID, connDeleteEvent, plugin.RiskDestructive, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleConnectionSessionStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canAccessConnection(ctx, user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	key := session.Key{ConnectionID: conn.ID, OwnerScope: user.ID}
	snap, ok := s.deps.Sessions.Status(key)
	if !ok {
		writeJSON(w, http.StatusOK, connectionSessionDTO{State: "idle"})
		return
	}
	writeJSON(w, http.StatusOK, s.connectionSessionDTO(snap))
}

func (s *Server) handleKeepaliveConnectionSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canAccessConnection(ctx, user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	res := resolved{user: user, conn: conn, route: plugin.Route{
		ID: "connection.session.keepalive", Permission: "connection.use", Risk: plugin.RiskSafe, AuditEvent: "connection.session.keepalive",
	}}
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		if snap, ok := s.deps.Sessions.Status(session.Key{ConnectionID: conn.ID, OwnerScope: user.ID}); ok {
			writeJSON(w, http.StatusOK, s.connectionSessionDTO(snap))
			return
		}
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, s.connectionSessionDTO(handle.Snapshot()))
}

func (s *Server) connectionSessionDTO(snap session.Snapshot) connectionSessionDTO {
	dto := connectionSessionDTO{
		State: string(snap.State), Reason: snap.Reason,
		Channels: snap.Channels, Streams: snap.Streams,
		LastSeen: snap.LastUsed.UTC().Format(time.RFC3339),
	}
	if !snap.LastHealthCheck.IsZero() {
		dto.LastHealthCheck = snap.LastHealthCheck.UTC().Format(time.RFC3339)
	}
	if snap.State != session.StateError && snap.Channels == 0 && snap.Streams == 0 {
		expires := time.Until(snap.LastUsed.Add(s.deps.Sessions.IdleTimeout()))
		if expires > 0 {
			dto.IdleExpiresIn = int64(expires.Seconds())
		}
	}
	return dto
}

func (s *Server) handleDisconnectConnectionSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canAccessConnection(ctx, user, conn) {
		s.auditConnEvent(ctx, user, conn.ID, connSessionDisconnectEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	s.deps.Sessions.Close(session.Key{ConnectionID: conn.ID, OwnerScope: user.ID})
	s.auditConnEvent(ctx, user, conn.ID, connSessionDisconnectEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// cleanupConnectionDependents removes the access-control state tied to a deleted
// connection so it can never be inherited by a future record: it drops any live
// agent tunnel, deletes sharing grants, and revokes outstanding enrollments.
// Best-effort — the connection is already gone, so failures are logged not fatal.
func (s *Server) cleanupConnectionDependents(ctx context.Context, connID string) {
	s.deps.Sessions.CloseConnection(connID)
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
