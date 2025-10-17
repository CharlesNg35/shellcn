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

const defaultTerminalType = "xterm-256color"

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
	templates      *services.ConnectionTemplateService
	vault          *services.VaultService
	activeSessions *services.ActiveSessionService
	lifecycle      *services.SessionLifecycleService
	recordings     *services.RecorderService
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
	templateSvc *services.ConnectionTemplateService,
	vaultSvc *services.VaultService,
	realtimeHub *realtime.Hub,
	activeSvc *services.ActiveSessionService,
	lifecycleSvc *services.SessionLifecycleService,
	recorderSvc *services.RecorderService,
	sftpChannels *services.SFTPChannelService,
	driverReg *drivers.Registry,
	checker *permissions.Checker,
	jwt *iauth.JWTService,
) *SSHSessionHandler {
	handler := &SSHSessionHandler{
		cfg:            cfg,
		connections:    connectionSvc,
		templates:      templateSvc,
		vault:          vaultSvc,
		activeSessions: activeSvc,
		lifecycle:      lifecycleSvc,
		recordings:     recorderSvc,
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

	sessionID := strings.TrimSpace(c.Query("session_id"))

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

	var prelaunchRecord *services.ActiveSessionRecord
	if sessionID != "" {
		record, ok := h.activeSessions.GetSession(sessionID)
		if !ok {
			response.Error(c, apperrors.ErrNotFound)
			return
		}
		if !strings.EqualFold(strings.TrimSpace(record.ConnectionID), connDTO.ID) {
			response.Error(c, apperrors.NewBadRequest("session does not belong to the requested connection"))
			return
		}
		if !strings.EqualFold(strings.TrimSpace(record.UserID), userID) {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		if !strings.EqualFold(strings.TrimSpace(record.ProtocolID), connDTO.ProtocolID) {
			response.Error(c, apperrors.NewBadRequest("session protocol mismatch"))
			return
		}
		prelaunchRecord = record
	}

	var (
		settings map[string]any
		host     string
		port     int
	)

	if h.templates != nil {
		config, cfgErr := h.templates.MaterialiseConfig(ctx, *connDTO)
		if cfgErr != nil {
			response.Error(c, cfgErr)
			return
		}
		if config != nil {
			settings = cloneMap(config.Settings)
			if len(config.Targets) > 0 {
				host = strings.TrimSpace(config.Targets[0].Host)
				port = config.Targets[0].Port
			}
		}
	}

	if settings == nil {
		settings = cloneMap(connDTO.Settings)
		var hostErr error
		host, port, hostErr = resolveHostPort(connDTO, settings)
		if hostErr != nil {
			response.Error(c, apperrors.NewBadRequest(hostErr.Error()))
			return
		}
	} else {
		if host == "" {
			host = strings.TrimSpace(stringFromAny(settings["host"]))
		}
		if port <= 0 {
			port = intFromAnyOrZero(settings["port"]) // fall back to stored value if template missed it
		}
		if host == "" {
			response.Error(c, apperrors.NewBadRequest("connection is missing host information"))
			return
		}
		if port <= 0 {
			port = 22
		}
	}

	settings["host"] = host
	settings["port"] = port

	sftpEnabled := true
	if h.cfg != nil {
		sftpEnabled = h.cfg.Protocols.SSH.EnableSFTPDefault
	}
	if value, ok := boolFromAny(settings["enable_sftp"]); ok {
		sftpEnabled = value
		delete(settings, "enable_sftp")
	}
	if strings.EqualFold(strings.TrimSpace(connDTO.ProtocolID), "sftp") {
		sftpEnabled = true
	}
	if prelaunchRecord != nil && prelaunchRecord.Metadata != nil {
		sftpEnabled = metadataBool(prelaunchRecord.Metadata, "sftp_enabled")
	}

	concurrencyLimit := sessionConcurrentLimit(settings, h.cfg)
	if prelaunchRecord != nil {
		concurrencyLimit = prelaunchRecord.ConcurrentLimit
	}

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
	if sessionID == "" {
		sessionID = uuid.NewString()
	}
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
	if prelaunchRecord != nil && strings.TrimSpace(userName) == "" {
		userName = strings.TrimSpace(prelaunchRecord.OwnerUserName)
	}

	var metadata map[string]any
	if prelaunchRecord != nil && prelaunchRecord.Metadata != nil {
		metadata = cloneMap(prelaunchRecord.Metadata)
		if _, exists := metadata["connection_id"]; !exists {
			metadata["connection_id"] = connDTO.ID
		}
	} else {
		metadata = map[string]any{
			"connection_id": connDTO.ID,
		}
	}

	terminalWidth := intFromAnyOrZero(settings["terminal_width"])
	if terminalWidth <= 0 {
		terminalWidth = 80
	}
	terminalHeight := intFromAnyOrZero(settings["terminal_height"])
	if terminalHeight <= 0 {
		terminalHeight = 24
	}
	terminalType := strings.TrimSpace(stringFromAny(settings["terminal_type"]))
	if terminalType == "" {
		terminalType = defaultTerminalType
	}

	policy := services.RecorderPolicy{
		Mode:           services.RecordingModeOptional,
		Storage:        "filesystem",
		RetentionDays:  0,
		RequireConsent: true,
	}
	if h.recordings != nil {
		policy = h.recordings.Policy()
	} else if h.cfg != nil {
		mode := strings.ToLower(strings.TrimSpace(h.cfg.Features.Recording.Mode))
		switch mode {
		case services.RecordingModeDisabled, services.RecordingModeForced, services.RecordingModeOptional:
			policy.Mode = mode
		default:
			policy.Mode = services.RecordingModeOptional
		}
		storage := strings.ToLower(strings.TrimSpace(h.cfg.Features.Recording.Storage))
		if storage == "" {
			storage = "filesystem"
		}
		policy.Storage = storage
		retention := h.cfg.Features.Recording.RetentionDays
		if retention < 0 {
			retention = 0
		}
		policy.RetentionDays = retention
		policy.RequireConsent = h.cfg.Features.Recording.RequireConsent
	}

	recordingEnabled := false
	switch policy.Mode {
	case services.RecordingModeForced:
		recordingEnabled = true
	case services.RecordingModeOptional:
		if value, ok := boolFromAny(settings["recording_enabled"]); ok {
			recordingEnabled = value
		}
	default:
		recordingEnabled = false
	}

	delete(settings, "recording_enabled")

	if prelaunchRecord == nil {
		metadata["terminal_width"] = terminalWidth
		metadata["terminal_height"] = terminalHeight
		metadata["terminal_type"] = terminalType
		metadata["sftp_enabled"] = sftpEnabled
		metadata["recording_enabled"] = recordingEnabled
		recordingActive := recordingEnabled && !strings.EqualFold(policy.Mode, services.RecordingModeDisabled)
		metadata["recording_active"] = recordingActive
		metadata["recording"] = map[string]any{
			"mode":            policy.Mode,
			"storage":         policy.Storage,
			"requested":       recordingEnabled,
			"active":          recordingActive,
			"retention_days":  policy.RetentionDays,
			"require_consent": policy.RequireConsent,
		}
	}

	metadata["sftp_enabled"] = sftpEnabled

	if connDTO.IdentityID != nil && strings.TrimSpace(*connDTO.IdentityID) != "" {
		metadata["identity_id"] = strings.TrimSpace(*connDTO.IdentityID)
	}

	var templateMeta map[string]any
	if connDTO.Metadata != nil {
		if raw, ok := connDTO.Metadata["connection_template"].(map[string]any); ok {
			templateMeta = cloneMap(raw)
			if fieldsRaw, ok := raw["fields"].(map[string]any); ok {
				templateMeta["fields"] = cloneMap(fieldsRaw)
			}
		}
	}
	if templateMeta != nil && len(templateMeta) == 0 {
		templateMeta = nil
	}

	capabilities, err := driverCapabilitiesMap(h.driverRegistry, connDTO.ProtocolID, sftpEnabled)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "resolve driver capabilities"))
		return
	}

	if prelaunchRecord == nil {
		if templateMeta != nil {
			metadata["template"] = templateMeta
		}
		if capabilities != nil {
			metadata["capabilities"] = capabilities
		}
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

	if prelaunchRecord == nil {
		startParams := services.StartSessionParams{
			SessionID:       sessionID,
			ConnectionID:    connDTO.ID,
			ConnectionName:  connDTO.Name,
			ProtocolID:      connDTO.ProtocolID,
			DescriptorID:    workspaceDescriptorID(connDTO.ProtocolID),
			OwnerUserID:     userID,
			OwnerUserName:   userName,
			TeamID:          connDTO.TeamID,
			Host:            host,
			Port:            port,
			Metadata:        metadata,
			Template:        templateMeta,
			Capabilities:    capabilities,
			ConcurrentLimit: concurrencyLimit,
			Actor:           sessionActor,
		}

		if _, err := h.lifecycle.StartSession(ctx, startParams); err != nil {
			h.handleRegisterError(c, err)
			return
		}
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

	if sftpEnabled && h.sftpChannels != nil {
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

	if h.activeSessions != nil {
		h.activeSessions.AttachHandle(sessionID, terminal)
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
				if h.recordings != nil && (event == "stdout" || event == "stderr") {
					if chunk := extractPayloadBytes(payload); len(chunk) > 0 {
						h.recordings.RecordStream(sessionID, event, chunk)
					}
				}
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

func sessionConcurrentLimit(settings map[string]any, cfg *app.Config) int {
	limit := 0
	if cfg != nil {
		limit = cfg.Features.Sessions.ConcurrentLimitDefault
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
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: true,
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

func boolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	case float64:
		return v != 0, true
	case int:
		return v != 0, true
	}
	return false, false
}

func extractPayloadBytes(payload any) []byte {
	switch v := payload.(type) {
	case []byte:
		if len(v) == 0 {
			return nil
		}
		out := make([]byte, len(v))
		copy(out, v)
		return out
	case map[string]any:
		if raw, ok := v["payload"].([]byte); ok && len(raw) > 0 {
			out := make([]byte, len(raw))
			copy(out, raw)
			return out
		}
	}
	return nil
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
