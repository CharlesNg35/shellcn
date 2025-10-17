package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/drivers"
	driversssh "github.com/charlesng35/shellcn/internal/drivers/ssh"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type ActiveSessionLaunchHandler struct {
	cfg         *app.Config
	connections *services.ConnectionService
	templates   *services.ConnectionTemplateService
	vault       *services.VaultService
	lifecycle   *services.SessionLifecycleService
	active      *services.ActiveSessionService
	recordings  *services.RecorderService
	drivers     *drivers.Registry
	checker     *permissions.Checker
	jwt         *iauth.JWTService
}

func NewActiveSessionLaunchHandler(
	cfg *app.Config,
	connections *services.ConnectionService,
	templates *services.ConnectionTemplateService,
	vault *services.VaultService,
	lifecycle *services.SessionLifecycleService,
	active *services.ActiveSessionService,
	recordings *services.RecorderService,
	driverReg *drivers.Registry,
	checker *permissions.Checker,
	jwt *iauth.JWTService,
) *ActiveSessionLaunchHandler {
	return &ActiveSessionLaunchHandler{
		cfg:         cfg,
		connections: connections,
		templates:   templates,
		vault:       vault,
		lifecycle:   lifecycle,
		active:      active,
		recordings:  recordings,
		drivers:     driverReg,
		checker:     checker,
		jwt:         jwt,
	}
}

type launchSessionRequest struct {
	ConnectionID   string         `json:"connection_id"`
	ProtocolID     string         `json:"protocol_id,omitempty"`
	FieldsOverride map[string]any `json:"fields_override,omitempty"`
}

type sessionTunnelInfo struct {
	URL       string            `json:"url"`
	Token     string            `json:"token"`
	Protocol  string            `json:"protocol"`
	ExpiresAt time.Time         `json:"expires_at"`
	Params    map[string]string `json:"params,omitempty"`
}

type workspaceDescriptorDTO struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name"`
	ProtocolID   string          `json:"protocol_id"`
	Icon         string          `json:"icon,omitempty"`
	DefaultRoute string          `json:"default_route"`
	Features     map[string]bool `json:"features,omitempty"`
}

type launchSessionResponse struct {
	Session          services.ActiveSessionRecord `json:"session"`
	Tunnel           sessionTunnelInfo            `json:"tunnel"`
	Descriptor       workspaceDescriptorDTO       `json:"descriptor"`
	TemplateMismatch bool                         `json:"template_mismatch,omitempty"`
}

// Launch creates a new active session for the supplied connection and returns
// the session metadata alongside tunnel credentials for the realtime bridge.
func (h *ActiveSessionLaunchHandler) Launch(c *gin.Context) {
	if h == nil || h.connections == nil || h.lifecycle == nil || h.active == nil || h.jwt == nil {
		response.Error(c, apperrors.New("session.launch.unavailable", "Session launch service unavailable", http.StatusServiceUnavailable))
		return
	}

	var req launchSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid launch payload"))
		return
	}

	connectionID := strings.TrimSpace(req.ConnectionID)
	if connectionID == "" {
		response.Error(c, apperrors.NewBadRequest("connection_id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)

	conn, err := h.connections.GetVisible(ctx, userID, connectionID, true, false)
	if err != nil {
		response.Error(c, err)
		return
	}

	protocolID := strings.TrimSpace(req.ProtocolID)
	if protocolID == "" {
		protocolID = conn.ProtocolID
	}

	if !strings.EqualFold(protocolID, conn.ProtocolID) {
		response.Error(c, apperrors.NewBadRequest("protocol mismatch for connection"))
		return
	}

	if ok, err := h.checkPermission(ctx, userID, connectionID, "connection.launch"); err != nil {
		response.Error(c, err)
		return
	} else if !ok {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	if ok, err := h.checkPermission(ctx, userID, connectionID, "protocol:"+protocolID+".connect"); err != nil {
		response.Error(c, err)
		return
	} else if !ok {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	if conn.IdentityID == nil || strings.TrimSpace(*conn.IdentityID) == "" {
		response.Error(c, apperrors.NewBadRequest("connection is missing a linked identity"))
		return
	}

	var claims *iauth.Claims
	if value, exists := c.Get(middleware.CtxClaimsKey); exists {
		if stored, ok := value.(*iauth.Claims); ok {
			claims = stored
		}
	}

	isRoot := false
	if claims != nil {
		isRoot = metadataBool(claims.Metadata, "is_root")
	}

	viewer, err := h.vault.ResolveViewer(ctx, userID, isRoot)
	if err != nil {
		response.Error(c, err)
		return
	}
	if _, err := h.vault.AuthorizeIdentityUse(ctx, viewer, strings.TrimSpace(*conn.IdentityID)); err != nil {
		response.Error(c, err)
		return
	}

	var baseConfig *services.ConnectionConfig
	if h.templates != nil {
		baseConfig, err = h.templates.MaterialiseConfig(ctx, *conn)
		if err != nil {
			response.Error(c, err)
			return
		}
	}

	materialised, templateMeta, templateMismatch, err := h.materialiseTemplate(ctx, conn, protocolID, req.FieldsOverride)
	if err != nil {
		response.Error(c, err)
		return
	}

	settings := cloneMap(conn.Settings)
	var host string
	var port int

	if baseConfig != nil {
		settings = cloneMap(baseConfig.Settings)
		if len(baseConfig.Targets) > 0 {
			host = strings.TrimSpace(baseConfig.Targets[0].Host)
			port = baseConfig.Targets[0].Port
		}
	}

	if materialised != nil {
		for key, value := range materialised.Settings {
			settings[key] = value
		}
		if len(materialised.Targets) > 0 {
			target := materialised.Targets[0]
			host = strings.TrimSpace(target.Host)
			port = target.Port
		}
	}

	if host == "" || port <= 0 {
		fallbackHost, fallbackPort, err := resolveHostPort(conn, settings)
		if err != nil {
			response.Error(c, apperrors.NewBadRequest(err.Error()))
			return
		}
		if host == "" {
			host = fallbackHost
		}
		if port <= 0 {
			port = fallbackPort
		}
	}

	settings["host"] = host
	settings["port"] = port

	sftpEnabled := h.resolveSFTP(settings, protocolID)

	concurrencyLimit := sessionConcurrentLimit(settings, h.cfg)

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

	recordingMetadata, recordingRequested, recordingActive := h.resolveRecording(settings)

	metadata := map[string]any{
		"connection_id":     conn.ID,
		"terminal_width":    terminalWidth,
		"terminal_height":   terminalHeight,
		"terminal_type":     terminalType,
		"sftp_enabled":      sftpEnabled,
		"recording":         recordingMetadata,
		"recording_enabled": recordingRequested,
		"recording_active":  recordingActive,
	}
	if baseConfig != nil && len(baseConfig.Metadata) > 0 {
		for key, value := range baseConfig.Metadata {
			if _, exists := metadata[key]; !exists {
				metadata[key] = value
			}
		}
	}

	if conn.IdentityID != nil {
		metadata["identity_id"] = strings.TrimSpace(*conn.IdentityID)
	}

	capabilities, err := driverCapabilitiesMap(h.drivers, protocolID, sftpEnabled)
	if err != nil {
		response.Error(c, err)
		return
	}
	if len(capabilities) > 0 {
		metadata["capabilities"] = capabilities
	}
	if len(templateMeta) > 0 {
		metadata["template"] = templateMeta
	}

	sessionID := uuidString()

	ownerName := h.userNameFromContext(c)

	startParams := services.StartSessionParams{
		SessionID:       sessionID,
		ConnectionID:    conn.ID,
		ConnectionName:  conn.Name,
		ProtocolID:      protocolID,
		DescriptorID:    workspaceDescriptorID(protocolID),
		OwnerUserID:     userID,
		OwnerUserName:   ownerName,
		TeamID:          conn.TeamID,
		Host:            host,
		Port:            port,
		Metadata:        metadata,
		Template:        templateMeta,
		Capabilities:    capabilities,
		ConcurrentLimit: concurrencyLimit,
		Actor: services.SessionActor{
			UserID:    userID,
			Username:  ownerName,
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		},
	}

	if _, err := h.lifecycle.StartSession(ctx, startParams); err != nil {
		h.handleRegisterError(c, err)
		return
	}

	record, ok := h.active.GetSession(sessionID)
	if !ok {
		response.Error(c, apperrors.New("session.launch.missing_record", "active session not registered", http.StatusInternalServerError))
		return
	}

	tunnel, err := h.buildTunnelResponse(c, record)
	if err != nil {
		response.Error(c, err)
		return
	}

	descriptor := h.buildDescriptor(protocolID, record.DescriptorID, conn.Name, sessionID)

	response.Success(c, http.StatusCreated, launchSessionResponse{
		Session:          *record,
		Tunnel:           tunnel,
		Descriptor:       descriptor,
		TemplateMismatch: templateMismatch,
	})
}

func (h *ActiveSessionLaunchHandler) checkPermission(ctx context.Context, userID, resourceID, permission string) (bool, error) {
	if h == nil || h.checker == nil {
		return true, nil
	}
	return h.checker.CheckResource(ctx, userID, "connection", resourceID, permission)
}

func (h *ActiveSessionLaunchHandler) materialiseTemplate(
	ctx context.Context,
	conn *services.ConnectionDTO,
	protocolID string,
	override map[string]any,
) (*services.MaterialisedConnection, map[string]any, bool, error) {
	if h == nil || h.templates == nil || conn == nil {
		return nil, nil, false, nil
	}

	template, err := h.templates.Resolve(ctx, protocolID)
	if err != nil || template == nil {
		return nil, nil, false, err
	}

	existingMeta := map[string]any{}
	fields := map[string]any{}

	if conn.Metadata != nil {
		if raw, ok := conn.Metadata["connection_template"]; ok {
			if meta, ok := raw.(map[string]any); ok {
				existingMeta = cloneMap(meta)
				if rawFields, ok := meta["fields"].(map[string]any); ok {
					fields = cloneMap(rawFields)
				}
			}
		}
	}

	for key, value := range override {
		fields[key] = value
	}

	materialised, err := h.templates.Materialise(template, fields)
	if err != nil {
		return nil, nil, false, err
	}

	if materialised == nil {
		return nil, existingMeta, false, nil
	}

	templateMeta := map[string]any{
		"driver_id": template.DriverID,
		"version":   template.Version,
		"fields":    cloneMap(materialised.Fields),
	}

	connectionVersion := strings.TrimSpace(stringFromAny(existingMeta["version"]))
	templateMismatch := connectionVersion != "" && !strings.EqualFold(connectionVersion, template.Version)
	if templateMismatch {
		templateMeta["version_mismatch"] = true
	}

	return materialised, templateMeta, templateMismatch, nil
}

func (h *ActiveSessionLaunchHandler) resolveSFTP(settings map[string]any, protocolID string) bool {
	sftpEnabled := true
	if h != nil && h.cfg != nil {
		sftpEnabled = h.cfg.Protocols.SSH.EnableSFTPDefault
	}
	if value, ok := boolFromAny(settings["enable_sftp"]); ok {
		sftpEnabled = value
		delete(settings, "enable_sftp")
	}
	if strings.EqualFold(protocolID, driversssh.DriverIDSFTP) {
		sftpEnabled = true
	}
	return sftpEnabled
}

func (h *ActiveSessionLaunchHandler) resolveRecording(settings map[string]any) (map[string]any, bool, bool) {
	policy := services.RecorderPolicy{
		Mode:           services.RecordingModeOptional,
		Storage:        "filesystem",
		RetentionDays:  0,
		RequireConsent: true,
	}

	if h != nil && h.recordings != nil {
		policy = h.recordings.Policy()
	} else if h != nil && h.cfg != nil {
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

	recordingActive := recordingEnabled && !strings.EqualFold(policy.Mode, services.RecordingModeDisabled)

	return map[string]any{
		"mode":            policy.Mode,
		"storage":         policy.Storage,
		"requested":       recordingEnabled,
		"active":          recordingActive,
		"retention_days":  policy.RetentionDays,
		"require_consent": policy.RequireConsent,
	}, recordingEnabled, recordingActive
}

func (h *ActiveSessionLaunchHandler) buildTunnelResponse(c *gin.Context, record *services.ActiveSessionRecord) (sessionTunnelInfo, error) {
	claims, _ := c.Get(middleware.CtxClaimsKey)
	var meta map[string]any
	if rawClaims, ok := claims.(*iauth.Claims); ok && rawClaims.Metadata != nil {
		meta = cloneMap(rawClaims.Metadata)
	}
	if meta == nil {
		meta = make(map[string]any)
	}

	meta["session_id"] = record.ID
	meta["connection_id"] = record.ConnectionID
	meta["tunnel"] = record.ProtocolID
	meta["protocol_id"] = record.ProtocolID

	token, err := h.jwt.GenerateAccessToken(iauth.AccessTokenInput{
		UserID:    record.OwnerUserID,
		SessionID: record.ID,
		Audience:  []string{"realtime"},
		Metadata:  meta,
	})
	if err != nil {
		return sessionTunnelInfo{}, err
	}

	expiresAt := time.Now().Add(h.jwt.AccessTokenTTL())

	params := map[string]string{
		"tunnel":        record.ProtocolID,
		"connection_id": record.ConnectionID,
		"session_id":    record.ID,
	}

	return sessionTunnelInfo{
		URL:       "/ws",
		Token:     token,
		Protocol:  record.ProtocolID,
		ExpiresAt: expiresAt,
		Params:    params,
	}, nil
}

func (h *ActiveSessionLaunchHandler) buildDescriptor(protocolID, descriptorID, connectionName, sessionID string) workspaceDescriptorDTO {
	var (
		icon        string
		displayName = strings.TrimSpace(connectionName)
	)

	if h != nil && h.drivers != nil {
		if driver, ok := h.drivers.Get(protocolID); ok {
			desc := driver.Descriptor()
			if desc.Title != "" {
				displayName = desc.Title
			}
			icon = desc.Icon
		}
	}

	if displayName == "" {
		displayName = strings.ToUpper(protocolID)
	}

	if descriptorID == "" {
		descriptorID = workspaceDescriptorID(protocolID)
	}

	return workspaceDescriptorDTO{
		ID:           descriptorID,
		DisplayName:  displayName,
		ProtocolID:   protocolID,
		Icon:         icon,
		DefaultRoute: "/active-sessions/" + sessionID,
	}
}

func (h *ActiveSessionLaunchHandler) handleRegisterError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, services.ErrActiveSessionExists):
		response.Error(c, apperrors.New("session.active_exists", "You already have an active session for this connection", http.StatusConflict).WithInternal(err))
	case errors.Is(err, services.ErrConcurrentLimitReached):
		response.Error(c, apperrors.New("session.concurrent_limit", "Concurrent session limit reached", http.StatusTooManyRequests).WithInternal(err))
	default:
		var limitErr *services.ConcurrentLimitError
		if errors.As(err, &limitErr) {
			response.Error(c, apperrors.New("session.concurrent_limit", limitErr.Error(), http.StatusTooManyRequests).WithInternal(err))
			return
		}
		response.Error(c, err)
	}
}

func (h *ActiveSessionLaunchHandler) userNameFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	claimsValue, _ := c.Get(middleware.CtxClaimsKey)
	if claims, ok := claimsValue.(*iauth.Claims); ok && claims != nil {
		if claims.Metadata != nil {
			if username, ok := claims.Metadata["username"].(string); ok && strings.TrimSpace(username) != "" {
				return strings.TrimSpace(username)
			}
		}
	}
	return strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
}

func uuidString() string {
	return uuid.NewString()
}
