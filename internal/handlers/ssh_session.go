package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/drivers"
	terminalbridge "github.com/charlesng35/shellcn/internal/handlers/terminal"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type sshTerminalHandle interface {
	drivers.SessionHandle
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	Resize(columns, rows int) error
}

// SSHSessionHandler handles websocket upgrades and bridges SSH sessions to clients.
type SSHSessionHandler struct {
	cfg            *app.Config
	connections    *services.ConnectionService
	vault          *services.VaultService
	activeSessions *services.ActiveSessionService
	lifecycle      *services.SessionLifecycleService
	sftpChannels   *services.SFTPChannelService
	driverRegistry *drivers.Registry
	checker        *permissions.Checker
	jwt            *iauth.JWTService
	hub            *realtime.Hub
}

// NewSSHSessionHandler constructs an SSH session handler when all dependencies are provided.
func NewSSHSessionHandler(
	cfg *app.Config,
	connectionSvc *services.ConnectionService,
	vaultSvc *services.VaultService,
	realtimeHub *realtime.Hub,
	activeSvc *services.ActiveSessionService,
	lifecycleSvc *services.SessionLifecycleService,
	sftpChannels *services.SFTPChannelService,
	driverReg *drivers.Registry,
	checker *permissions.Checker,
	jwt *iauth.JWTService,
) *SSHSessionHandler {
	handler := &SSHSessionHandler{
		cfg:            cfg,
		connections:    connectionSvc,
		vault:          vaultSvc,
		activeSessions: activeSvc,
		lifecycle:      lifecycleSvc,
		sftpChannels:   sftpChannels,
		driverRegistry: driverReg,
		checker:        checker,
		jwt:            jwt,
		hub:            realtimeHub,
	}

	return handler
}

// ServeTunnel bridges an SSH session over WebSocket using prevalidated claims.
func (h *SSHSessionHandler) ServeTunnel(c *gin.Context, claims *iauth.Claims) {
	if h == nil || h.driverRegistry == nil || h.connections == nil || h.vault == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	if h.cfg != nil && !h.cfg.Protocols.SSH.Enabled {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	if claims == nil || strings.TrimSpace(claims.UserID) == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}
	userID := strings.TrimSpace(claims.UserID)

	connectionID := strings.TrimSpace(c.Query("connection_id"))
	if connectionID == "" {
		connectionID = strings.TrimSpace(c.Param("connectionID"))
	}
	if connectionID == "" {
		response.Error(c, apperrors.NewBadRequest("connection id is required"))
		return
	}

	ctx := requestContext(c)
	connDTO, err := h.connections.GetVisible(ctx, userID, connectionID, true, false)
	if err != nil {
		response.Error(c, err)
		return
	}
	if !strings.EqualFold(connDTO.ProtocolID, "ssh") {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	if ok, permErr := h.checkPermission(ctx, userID, connectionID, "connection.launch"); permErr != nil {
		response.Error(c, permErr)
		return
	} else if !ok {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	if ok, permErr := h.checkPermission(ctx, userID, connectionID, "protocol:ssh.connect"); permErr != nil {
		response.Error(c, permErr)
		return
	} else if !ok {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	settings := cloneMap(connDTO.Settings)
	host, port, hostErr := resolveHostPort(connDTO, settings)
	if hostErr != nil {
		response.Error(c, apperrors.NewBadRequest(hostErr.Error()))
		return
	}

	if _, exists := settings["host"]; !exists {
		settings["host"] = host
	}
	settings["port"] = port

	concurrencyLimit := h.sessionConcurrentLimit(settings)

	var identitySecret map[string]any
	isRoot := metadataBool(claims.Metadata, "is_root")
	if connDTO.IdentityID != nil && strings.TrimSpace(*connDTO.IdentityID) != "" {
		viewer, err := h.vault.ResolveViewer(ctx, userID, isRoot)
		if err != nil {
			response.Error(c, err)
			return
		}
		identitySecret, err = h.vault.LoadIdentitySecret(ctx, viewer, *connDTO.IdentityID)
		if err != nil {
			response.Error(c, err)
			return
		}
	} else {
		response.Error(c, apperrors.NewBadRequest("connection is missing a linked identity"))
		return
	}

	secret := cloneMap(identitySecret)
	sessionID := uuid.NewString()
	secret["session_id"] = sessionID

	driver, ok := h.driverRegistry.Get(connDTO.ProtocolID)
	if !ok {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	launcher, ok := driver.(drivers.Launcher)
	if !ok {
		response.Error(c, apperrors.New("protocol.unsupported", "protocol driver does not support launching sessions", http.StatusNotImplemented))
		return
	}

	userName := metadataString(claims.Metadata, "username")
	if userName == "" {
		userName = metadataString(claims.Metadata, "email")
	}
	if userName == "" {
		userName = userID
	}

	metadata := map[string]any{
		"connection_id": connDTO.ID,
	}
	if connDTO.IdentityID != nil && strings.TrimSpace(*connDTO.IdentityID) != "" {
		metadata["identity_id"] = strings.TrimSpace(*connDTO.IdentityID)
	}
	sessionActor := services.SessionActor{
		UserID:    userID,
		Username:  userName,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	if h.lifecycle == nil {
		response.Error(c, apperrors.New("session.lifecycle_unavailable", "session lifecycle service unavailable", http.StatusInternalServerError))
		return
	}

	startParams := services.StartSessionParams{
		SessionID:       sessionID,
		ConnectionID:    connDTO.ID,
		ConnectionName:  connDTO.Name,
		ProtocolID:      connDTO.ProtocolID,
		OwnerUserID:     userID,
		OwnerUserName:   userName,
		TeamID:          connDTO.TeamID,
		Host:            host,
		Port:            port,
		Metadata:        metadata,
		ConcurrentLimit: concurrencyLimit,
		Actor:           sessionActor,
	}

	if _, err := h.lifecycle.StartSession(ctx, startParams); err != nil {
		h.handleRegisterError(c, err)
		return
	}

	h.broadcastTerminal(sessionID, "opened", map[string]any{
		"connection_id": connDTO.ID,
		"user_id":       userID,
	})
	defer h.broadcastTerminal(sessionID, "closed", map[string]any{
		"connection_id": connDTO.ID,
		"user_id":       userID,
	})

	closeStatus := services.SessionStatusClosed
	closeReason := "completed"
	defer func() {
		if h.lifecycle != nil {
			_ = h.lifecycle.CloseSession(context.Background(), services.CloseSessionParams{
				SessionID: sessionID,
				Status:    closeStatus,
				Reason:    closeReason,
				Actor:     sessionActor,
			})
		}
	}()

	req := drivers.SessionRequest{
		ConnectionID: connDTO.ID,
		ProtocolID:   connDTO.ProtocolID,
		UserID:       userID,
		Settings:     settings,
		Secret:       secret,
	}

	handle, err := launcher.Launch(ctx, req)
	if err != nil {
		closeStatus = services.SessionStatusFailed
		closeReason = err.Error()
		response.Error(c, apperrors.Wrap(err, "failed to launch ssh session"))
		return
	}

	if h.sftpChannels != nil {
		if provider, ok := handle.(services.SFTPProvider); ok {
			if err := h.sftpChannels.Attach(sessionID, provider); err != nil {
				closeStatus = services.SessionStatusFailed
				closeReason = "sftp channel registration failed"
				_ = handle.Close(context.Background())
				response.Error(c, apperrors.Wrap(err, "register sftp channel"))
				return
			}
			defer h.sftpChannels.Detach(sessionID)
		}
	}

	terminal, ok := handle.(sshTerminalHandle)
	if !ok {
		_ = handle.Close(context.Background())
		closeStatus = services.SessionStatusFailed
		closeReason = "incompatible session handle"
		response.Error(c, apperrors.New("session.handle_incompatible", "ssh driver returned incompatible session handle", http.StatusInternalServerError))
		return
	}

	wsConn, err := h.upgradeConnection(c)
	if err != nil {
		_ = terminal.Close(context.Background())
		closeStatus = services.SessionStatusFailed
		closeReason = "websocket upgrade failed"
		return
	}

	defer wsConn.Close()
	defer terminal.Close(context.Background())

	ready := map[string]any{
		"type":          "ready",
		"session_id":    sessionID,
		"connection_id": connDTO.ID,
		"connection":    connDTO.Name,
	}
	if err := wsConn.WriteJSON(ready); err != nil {
		closeStatus = services.SessionStatusFailed
		closeReason = "websocket ready write failed"
		return
	}

	h.broadcastTerminal(sessionID, "ready", map[string]any{
		"connection_id": connDTO.ID,
		"connection":    connDTO.Name,
		"user_id":       userID,
	})

	bridgeErr := terminalbridge.Run(c.Request.Context(), terminalbridge.Config{
		Conn:      wsConn,
		SessionID: sessionID,
		Streams:   terminal,
		Callbacks: terminalbridge.Callbacks{
			OnHeartbeat: func() {
				if h.lifecycle != nil {
					_ = h.lifecycle.Heartbeat(context.Background(), sessionID)
				} else if h.activeSessions != nil {
					h.activeSessions.Heartbeat(sessionID)
				}
			},
			OnEvent: func(event string, payload any) {
				h.handleTerminalEvent(sessionID, connDTO.ID, event, payload)
			},
			OnError: func(err error) {
				closeStatus = services.SessionStatusFailed
				closeReason = err.Error()
				h.broadcastTerminal(sessionID, "error", map[string]any{
					"connection_id": connDTO.ID,
					"message":       err.Error(),
				})
			},
		},
	})
	if bridgeErr != nil && !isNormalSocketClose(bridgeErr) && !stdErrors.Is(bridgeErr, context.Canceled) {
		closeStatus = services.SessionStatusFailed
		closeReason = bridgeErr.Error()
	}
}

func (h *SSHSessionHandler) checkPermission(ctx context.Context, userID, resourceID, permission string) (bool, error) {
	if h.checker == nil {
		return true, nil
	}
	return h.checker.CheckResource(ctx, userID, "connection", resourceID, permission)
}

func (h *SSHSessionHandler) sessionConcurrentLimit(settings map[string]any) int {
	limit := 0
	if h.cfg != nil {
		limit = h.cfg.Features.Sessions.ConcurrentLimitDefault
	}

	if v, ok := intFromAny(settings["concurrent_limit"]); ok {
		limit = v
		delete(settings, "concurrent_limit")
	}
	if limit < 0 {
		limit = 0
	}
	return limit
}

func (h *SSHSessionHandler) handleTerminalEvent(sessionID, connectionID, event string, payload any) {
	if h == nil {
		return
	}

	if data, ok := payload.(map[string]any); ok {
		data["connection_id"] = connectionID
		if raw, exists := data["payload"].([]byte); exists {
			data["payload"] = base64.StdEncoding.EncodeToString(raw)
			data["encoding"] = "base64"
		}
		payload = data
	} else if chunk, ok := payload.([]byte); ok {
		payload = map[string]any{
			"connection_id": connectionID,
			"payload":       base64.StdEncoding.EncodeToString(chunk),
			"encoding":      "base64",
		}
	}

	h.broadcastTerminal(sessionID, event, payload)
}

func (h *SSHSessionHandler) handleRegisterError(c *gin.Context, err error) {
	switch {
	case stdErrors.Is(err, services.ErrConcurrentLimitReached):
		response.Error(c, apperrors.New("session.concurrent_limit", "Concurrent session limit reached", http.StatusTooManyRequests).WithInternal(err))
	case stdErrors.Is(err, services.ErrActiveSessionExists):
		response.Error(c, apperrors.New("session.active_exists", "You already have an active session for this connection", http.StatusConflict).WithInternal(err))
	default:
		response.Error(c, apperrors.Wrap(err, "register active session"))
	}
}

func (h *SSHSessionHandler) upgradeConnection(c *gin.Context) (*websocket.Conn, error) {
	if h != nil && h.hub != nil {
		return h.hub.Upgrade(c.Writer, c.Request)
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true
			}
			originHost := hostWithoutPort(origin)
			requestHost := hostWithoutPort(r.Host)
			return originHost == requestHost || isLoopback(originHost)
		},
	}
	return upgrader.Upgrade(c.Writer, c.Request, nil)
}

type outboundMessage struct {
	messageType int
	payload     []byte
}

func metadataBool(meta map[string]any, key string) bool {
	if len(meta) == 0 {
		return false
	}
	if raw, ok := meta[key]; ok {
		switch v := raw.(type) {
		case bool:
			return v
		case float64:
			return v != 0
		case string:
			parsed, err := strconv.ParseBool(v)
			if err == nil {
				return parsed
			}
		}
	}
	return false
}

func metadataString(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	if raw, ok := meta[key]; ok {
		if str, ok := raw.(string); ok {
			return str
		}
	}
	return ""
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(source))
	for k, v := range source {
		out[k] = v
	}
	return out
}

func resolveHostPort(dto *services.ConnectionDTO, settings map[string]any) (string, int, error) {
	host := strings.TrimSpace(stringFromAny(settings["host"]))
	port := intFromAnyOrZero(settings["port"])

	if host == "" && len(dto.Targets) > 0 {
		host = strings.TrimSpace(dto.Targets[0].Host)
		if port == 0 {
			port = dto.Targets[0].Port
		}
	}

	if host == "" {
		return "", 0, fmt.Errorf("connection is missing host information")
	}
	if port <= 0 {
		port = 22
	}

	return host, port, nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func intFromAnyOrZero(value any) int {
	if v, ok := intFromAny(value); ok {
		return v
	}
	return 0
}

func (h *SSHSessionHandler) broadcastTerminal(sessionID string, event string, payload any) {
	if h == nil || h.hub == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	var data map[string]any
	switch v := payload.(type) {
	case map[string]any:
		data = v
		if _, ok := data["session_id"]; !ok {
			data["session_id"] = sessionID
		}
	case nil:
		data = map[string]any{"session_id": sessionID}
	default:
		data = map[string]any{
			"session_id": sessionID,
			"payload":    v,
		}
	}

	if raw, ok := data["payload"].([]byte); ok {
		if len(raw) == 0 {
			return
		}
		data["payload"] = base64.StdEncoding.EncodeToString(raw)
		data["encoding"] = "base64"
	}

	h.hub.BroadcastStream(realtime.StreamSSHTerminal, realtime.Message{
		Event: event,
		Data:  data,
	})
}

func isNormalSocketClose(err error) bool {
	if err == nil {
		return true
	}
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway)
}

// hostWithoutPort mirrors realtime's origin validation.
func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		req, err := http.NewRequest(http.MethodGet, host, nil)
		if err == nil {
			return hostWithoutPort(req.URL.Host)
		}
	}

	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}

	return host
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
