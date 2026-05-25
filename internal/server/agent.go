package server

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
)

const (
	agentEnrollEvent     = "agent.enrollment.create"
	agentConnectEvent    = "agent.connect"
	agentDisconnectEvent = "agent.disconnect"
)

// canManageConnection reports whether the user may enroll/operate an agent for a
// connection: owner, admin, or a holder of a manage grant.
func (s *Server) canManageConnection(ctx context.Context, user models.User, conn models.Connection) bool {
	if user.HasRole(models.RoleAdmin) || conn.OwnerID == user.ID {
		return true
	}
	g, err := s.deps.Store.Grants.Get(ctx, conn.ID, user.ID)
	return err == nil && g.Access == models.AccessManage
}

func (s *Server) agentConnectURL(r *http.Request) string {
	scheme := "ws"
	if isTLS(r) {
		scheme = "wss"
	}
	return scheme + "://" + r.Host + "/api/agent/connect"
}

func (s *Server) auditAgentEvent(ctx context.Context, user models.User, connectionID, event string, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User:         user,
		Event:        event,
		ConnectionID: connectionID,
		RouteID:      event,
		Risk:         string(plugin.RiskPrivileged),
		Result:       result,
		Err:          err,
	})
}

func (s *Server) handleCreateEnrollment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canManageConnection(ctx, user, conn) {
		s.auditAgentEvent(ctx, user, conn.ID, agentEnrollEvent, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	enr, err := s.deps.Enrollments.Create(ctx, conn.ID, s.agentConnectURL(r))
	if err != nil {
		s.auditAgentEvent(ctx, user, conn.ID, agentEnrollEvent, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAgentEvent(ctx, user, conn.ID, agentEnrollEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusCreated, enr)
}

func (s *Server) handleAgentState(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, s.deps.Enrollments.State(ctx, conn.ID))
}

// handleAgentConnect is the agent tunnel endpoint. The agent authenticates with
// its enrollment token in the first message (not the URL); on success the gateway
// registers the connection's dialer and serves the multiplexed tunnel.
func (s *Server) handleAgentConnect(w http.ResponseWriter, r *http.Request) {
	// No InsecureSkipVerify: the CLI agent sends no Origin header (accepted by
	// default), while a browser's cross-origin upgrade carries a mismatched
	// Origin and is rejected. The enrollment token in the first message is the
	// authenticator.
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}

	handshakeCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var hello transport.AgentHello
	if err := wsjson.Read(handshakeCtx, c, &hello); err != nil {
		_ = c.Close(websocket.StatusPolicyViolation, "handshake required")
		return
	}

	connID, proxy, err := s.deps.Enrollments.Redeem(r.Context(), hello.Token)
	if err != nil {
		_ = wsjson.Write(handshakeCtx, c, transport.AgentConnectResponse{OK: false, Error: "enrollment rejected"})
		_ = c.Close(websocket.StatusPolicyViolation, "enrollment rejected")
		return
	}

	resp := transport.AgentConnectResponse{
		OK:    true,
		Proxy: transport.AgentProxyTarget{Mode: string(proxy.Mode), Address: proxy.Address},
	}
	if err := wsjson.Write(handshakeCtx, c, resp); err != nil {
		_ = c.Close(websocket.StatusInternalError, "handshake failed")
		return
	}

	agentUser := models.User{ID: "agent", Username: "shellcn-agent"}
	s.auditAgentEvent(r.Context(), agentUser, connID, agentConnectEvent, models.AuditAllowed, nil)
	s.deps.Logger.Info("agent tunnel online", "connection", connID, "mode", proxy.Mode)
	tunnelErr := transport.ServeGatewayTunnel(c, connID, s.deps.Tunnels)
	s.deps.Enrollments.MarkOffline(r.Context(), connID)
	result := models.AuditAllowed
	if tunnelErr != nil {
		result = models.AuditError
	}
	s.auditAgentEvent(r.Context(), agentUser, connID, agentDisconnectEvent, result, tunnelErr)
	s.deps.Logger.Info("agent tunnel offline", "connection", connID)
}
