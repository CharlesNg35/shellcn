package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ProtocolSettingsHandler exposes admin endpoints for protocol configuration.
type ProtocolSettingsHandler struct {
	service *services.ProtocolSettingsService
	checker *permissions.Checker
}

// NewProtocolSettingsHandler constructs a handler once dependencies are supplied.
func NewProtocolSettingsHandler(service *services.ProtocolSettingsService, checker *permissions.Checker) *ProtocolSettingsHandler {
	return &ProtocolSettingsHandler{
		service: service,
		checker: checker,
	}
}

// GET /api/settings/protocols/ssh
func (h *ProtocolSettingsHandler) GetSSHSettings(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	if allowed, err := h.authorize(ctx, userID); err != nil {
		response.Error(c, err)
		return
	} else if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	settings, err := h.service.GetSSHSettings(ctx)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "fetch ssh protocol settings"))
		return
	}
	response.Success(c, http.StatusOK, settings)
}

// PUT /api/settings/protocols/ssh
func (h *ProtocolSettingsHandler) UpdateSSHSettings(c *gin.Context) {
	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	var payload updateSSHSettingsRequest
	if !bindAndValidate(c, &payload) {
		return
	}

	ctx := requestContext(c)
	if allowed, err := h.authorize(ctx, userID); err != nil {
		response.Error(c, err)
		return
	} else if !allowed {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	actorMeta, _ := auditctx.FromContext(c.Request.Context())
	actor := services.SessionActor{
		UserID:    userID,
		Username:  actorMeta.Username,
		IPAddress: actorMeta.IPAddress,
		UserAgent: actorMeta.UserAgent,
	}

	settings, err := h.service.UpdateSSHSettings(ctx, actor, services.UpdateSSHSettingsInput{
		Session: services.SessionSettingsInput{
			ConcurrentLimit:    payload.Session.ConcurrentLimit,
			IdleTimeoutMinutes: payload.Session.IdleTimeoutMinutes,
			EnableSFTP:         payload.Session.EnableSFTP,
		},
		Terminal: services.TerminalSettingsInput{
			ThemeMode:   payload.Terminal.ThemeMode,
			FontFamily:  payload.Terminal.FontFamily,
			FontSize:    payload.Terminal.FontSize,
			Scrollback:  payload.Terminal.Scrollback,
			EnableWebGL: payload.Terminal.EnableWebGL,
		},
		Recording: services.RecordingSettingsInput{
			Mode:           payload.Recording.Mode,
			Storage:        payload.Recording.Storage,
			RetentionDays:  payload.Recording.RetentionDays,
			RequireConsent: payload.Recording.RequireConsent,
		},
		Collaboration: services.CollaborationSettingsInput{
			AllowSharing:          payload.Collaboration.AllowSharing,
			RestrictWriteToAdmins: payload.Collaboration.RestrictWriteToAdmins,
		},
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "update ssh protocol settings"))
		return
	}

	response.Success(c, http.StatusOK, settings)
}

func (h *ProtocolSettingsHandler) authorize(ctx context.Context, userID string) (bool, error) {
	if h.checker == nil {
		return true, nil
	}
	return h.checker.Check(ctx, userID, "protocol:ssh.record")
}

type updateSSHSettingsRequest struct {
	Session       sessionSettingsPayload       `json:"session" binding:"required"`
	Terminal      terminalSettingsPayload      `json:"terminal" binding:"required"`
	Recording     recordingSettingsPayload     `json:"recording" binding:"required"`
	Collaboration collaborationSettingsPayload `json:"collaboration" binding:"required"`
}

type sessionSettingsPayload struct {
	ConcurrentLimit    int  `json:"concurrent_limit" binding:"min=0,max=1000"`
	IdleTimeoutMinutes int  `json:"idle_timeout_minutes" binding:"min=0,max=10080"`
	EnableSFTP         bool `json:"enable_sftp"`
}

type terminalSettingsPayload struct {
	ThemeMode   string `json:"theme_mode" binding:"required,oneof=auto force_dark force_light"`
	FontFamily  string `json:"font_family" binding:"required,max=128"`
	FontSize    int    `json:"font_size" binding:"min=8,max=96"`
	Scrollback  int    `json:"scrollback_limit" binding:"min=200,max=10000"`
	EnableWebGL bool   `json:"enable_webgl"`
}

type recordingSettingsPayload struct {
	Mode           string `json:"mode" binding:"required,oneof=disabled optional forced"`
	Storage        string `json:"storage" binding:"required,oneof=filesystem s3"`
	RetentionDays  int    `json:"retention_days" binding:"min=0,max=3650"`
	RequireConsent bool   `json:"require_consent"`
}

type collaborationSettingsPayload struct {
	AllowSharing          bool `json:"allow_sharing"`
	RestrictWriteToAdmins bool `json:"restrict_write_to_admins"`
}
