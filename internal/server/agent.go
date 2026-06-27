package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	agentEnrollEvent     = "agent.enrollment.create"
	agentConnectEvent    = "agent.connect"
	agentDisconnectEvent = "agent.disconnect"
	agentArtifactEvent   = "agent.artifact.fetch"
	// artifactTicketRoute is the synthetic ticket scope for install-artifact
	// fetches — generic, not tied to any plugin or artifact kind.
	artifactTicketRoute = "agent.artifact"
)

var errAgentHandshakeRequired = errors.New("agent handshake required")

// canAdminConnection reports whether the user may mutate the saved connection
// record or its control-plane lifecycle. Admin confers no implicit access.
func (s *Server) canAdminConnection(user models.User, conn models.Connection) bool {
	return conn.OwnerID == user.ID
}

func (s *Server) agentConnectURL(r *http.Request) string {
	scheme := "ws"
	if isTLS(r) {
		scheme = "wss"
	}
	return scheme + "://" + gatewayConnectHost(r) + "/api/agent/connect"
}

func gatewayConnectHost(r *http.Request) string {
	host := requestHost(r)
	if !isLoopbackRequestHost(host) {
		return host
	}
	port := requestLocalPort(r)
	if port == "" {
		return host
	}
	hostname := hostWithoutPort(host)
	if hostname == "" {
		return host
	}
	if _, currentPort, err := net.SplitHostPort(host); err == nil && currentPort == port {
		return host
	}
	return net.JoinHostPort(hostname, port)
}

func requestLocalPort(r *http.Request) string {
	addr, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if addr == nil {
		return ""
	}
	if tcp, ok := addr.(*net.TCPAddr); ok && tcp.Port > 0 {
		return strconv.Itoa(tcp.Port)
	}
	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return ""
	}
	return port
}

func isLoopbackRequestHost(host string) bool {
	hostname := hostWithoutPort(host)
	if hostname == "localhost" {
		return true
	}
	ip := net.ParseIP(hostname)
	return ip != nil && ip.IsLoopback()
}

func hostWithoutPort(host string) string {
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return hostname
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
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
	if !s.canAdminConnection(user, conn) {
		s.auditAgentEvent(ctx, user, conn.ID, agentEnrollEvent, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	enr, err := s.deps.Enrollments.Create(ctx, conn.ID, s.agentConnectURL(r), s.artifactURLMinter(r, conn.ID))
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
	if !s.canAdminConnection(user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	state := s.deps.Enrollments.State(ctx, conn.ID)
	if state.Status == string(models.EnrollmentOnline) && !s.agentReachable(ctx, conn.ID) {
		s.deps.Enrollments.MarkOffline(ctx, conn.ID)
		state = s.deps.Enrollments.State(ctx, conn.ID)
	}
	writeJSON(w, http.StatusOK, state)
}

// artifactURLMinter mints a single-use signed ticket and builds the public fetch
// URL for a URL-delivered artifact (nil when artifact delivery is not wired).
func (s *Server) artifactURLMinter(r *http.Request, connID string) service.ArtifactURLFunc {
	if s.deps.ArtifactTickets == nil {
		return nil
	}
	return func(enrollmentID, kind string) (string, error) {
		ticket, _ := s.deps.ArtifactTickets.Mint(auth.TicketScope{
			ConnectionID: connID,
			RouteID:      artifactTicketRoute,
			Params:       map[string]string{"enrollmentId": enrollmentID, "kind": kind},
		})
		scheme := "http"
		if isTLS(r) {
			scheme = "https"
		}
		u := url.URL{
			Scheme:   scheme,
			Host:     gatewayConnectHost(r),
			Path:     "/api/connections/" + url.PathEscape(connID) + "/agent/enrollments/" + url.PathEscape(enrollmentID) + "/artifacts/" + url.PathEscape(kind),
			RawQuery: url.Values{"ticket": {ticket}}.Encode(),
		}
		return u.String(), nil
	}
}

// handleFetchArtifact serves a URL-delivered artifact's body, authorized solely
// by a single-use signed ticket (the token is minted into the body here).
func (s *Server) handleFetchArtifact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	connID := chi.URLParam(r, "id")
	enrollmentID := chi.URLParam(r, "enrollmentId")
	kind := chi.URLParam(r, "kind")

	scope := auth.TicketScope{
		ConnectionID: connID,
		RouteID:      artifactTicketRoute,
		Params:       map[string]string{"enrollmentId": enrollmentID, "kind": kind},
	}
	if err := s.deps.ArtifactTickets.Redeem(r.URL.Query().Get("ticket"), scope); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}

	content, err := s.deps.Enrollments.RenderArtifactContent(ctx, connID, enrollmentID, kind, s.agentConnectURL(r))
	if err != nil {
		s.auditAgentEvent(ctx, artifactFetchUser, connID, agentArtifactEvent, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAgentEvent(ctx, artifactFetchUser, connID, agentArtifactEvent, models.AuditAllowed, nil)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, content)
}

// artifactFetchUser labels the unauthenticated, ticket-authorized artifact fetch
// in the audit log.
var artifactFetchUser = models.User{ID: "agent", Username: "artifact-fetch"}

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
		agentUser := models.User{ID: "agent", Username: app.AgentUsername}
		s.auditAgentEvent(r.Context(), agentUser, "", agentConnectEvent, models.AuditDenied, errAgentHandshakeRequired)
		_ = c.Close(websocket.StatusPolicyViolation, "handshake required")
		return
	}

	connID, proxy, err := s.deps.Enrollments.Redeem(r.Context(), hello.Token)
	if err != nil {
		agentUser := models.User{ID: "agent", Username: app.AgentUsername}
		s.auditAgentEvent(r.Context(), agentUser, "", agentConnectEvent, models.AuditDenied, err)
		_ = wsjson.Write(handshakeCtx, c, transport.AgentConnectResponse{OK: false, Error: "enrollment rejected"})
		_ = c.Close(websocket.StatusPolicyViolation, "enrollment rejected")
		return
	}

	// Per-stream forwarding needs both the plugin to opt in and the agent to support it.
	forward := proxy.Forward && hello.Forward
	resp := transport.AgentConnectResponse{
		OK: true,
		Proxy: transport.AgentProxyTarget{
			Mode:      string(proxy.Mode),
			Address:   proxy.Address,
			TokenFile: proxy.TokenFile,
			CAFile:    proxy.CAFile,
			Forward:   forward,
		},
	}
	agentUser := models.User{ID: "agent", Username: app.AgentUsername}
	connected := false
	releasedActive, tunnelErr := transport.ServeGatewayTunnel(r.Context(), c, connID, s.deps.Tunnels, forward, func() error {
		if err := wsjson.Write(handshakeCtx, c, resp); err != nil {
			_ = c.Close(websocket.StatusInternalError, "handshake failed")
			return err
		}
		connected = true
		s.auditAgentEvent(r.Context(), agentUser, connID, agentConnectEvent, models.AuditAllowed, nil)
		s.deps.Logger.Info("agent tunnel online", "connection", connID, "mode", proxy.Mode)
		return nil
	})
	teardownCtx, teardownCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer teardownCancel()
	if releasedActive {
		s.deps.Enrollments.MarkOffline(teardownCtx, connID)
	}
	if !connected {
		return
	}
	result := models.AuditAllowed
	if tunnelErr != nil {
		result = models.AuditError
	}
	s.auditAgentEvent(teardownCtx, agentUser, connID, agentDisconnectEvent, result, tunnelErr)
	s.deps.Logger.Info("agent tunnel offline", "connection", connID)
}
