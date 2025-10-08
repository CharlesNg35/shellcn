package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type AuthProviderHandler struct {
	svc *services.AuthProviderService
}

func NewAuthProviderHandler(svc *services.AuthProviderService) *AuthProviderHandler {
	return &AuthProviderHandler{svc: svc}
}

// GET /api/auth/providers/all
func (h *AuthProviderHandler) ListAll(c *gin.Context) {
	providers, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, providers)
}

// GET /api/auth/providers/enabled
func (h *AuthProviderHandler) GetEnabled(c *gin.Context) {
	providers, err := h.svc.GetEnabled(c.Request.Context())
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, providers)
}

// GET /api/auth/providers (public)
func (h *AuthProviderHandler) ListPublic(c *gin.Context) {
	providers, err := h.svc.GetEnabledPublic(c.Request.Context())
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, providers)
}

// POST /api/auth/providers/local/settings
func (h *AuthProviderHandler) UpdateLocalSettings(c *gin.Context) {
	var body struct {
		AllowRegistration        bool `json:"allow_registration"`
		RequireEmailVerification bool `json:"require_email_verification"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	if err := h.svc.UpdateLocalSettings(c.Request.Context(), body.AllowRegistration, body.RequireEmailVerification); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// POST /api/auth/providers/:type/enable
func (h *AuthProviderHandler) SetEnabled(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	if err := h.svc.SetEnabled(c.Request.Context(), c.Param("type"), body.Enabled); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// POST /api/auth/providers/:type/test
func (h *AuthProviderHandler) TestConnection(c *gin.Context) {
	if err := h.svc.TestConnection(c.Request.Context(), c.Param("type")); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"ok": true})
}

// POST /api/auth/providers/:type/configure
func (h *AuthProviderHandler) Configure(c *gin.Context) {
	var actor string
	if v, ok := c.Get("userID"); ok {
		actor, _ = v.(string)
	}
	ptype := c.Param("type")

	switch ptype {
	case "oidc":
		var cfg models.OIDCConfig
		var body struct {
			Enabled bool              `json:"enabled"`
			Config  models.OIDCConfig `json:"config"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Error(c, errors.ErrBadRequest)
			return
		}
		cfg = body.Config
		if err := h.svc.ConfigureOIDC(c.Request.Context(), cfg, body.Enabled, actor); err != nil {
			response.Error(c, errors.ErrInternalServer)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"updated": true})
	case "oauth2":
		var body struct {
			Enabled bool                `json:"enabled"`
			Config  models.OAuth2Config `json:"config"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Error(c, errors.ErrBadRequest)
			return
		}
		if err := h.svc.ConfigureOAuth2(c.Request.Context(), body.Config, body.Enabled, actor); err != nil {
			response.Error(c, errors.ErrInternalServer)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"updated": true})
	case "saml":
		var body struct {
			Enabled bool              `json:"enabled"`
			Config  models.SAMLConfig `json:"config"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Error(c, errors.ErrBadRequest)
			return
		}
		if err := h.svc.ConfigureSAML(c.Request.Context(), body.Config, body.Enabled, actor); err != nil {
			response.Error(c, errors.ErrInternalServer)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"updated": true})
	case "ldap":
		var body struct {
			Enabled bool              `json:"enabled"`
			Config  models.LDAPConfig `json:"config"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Error(c, errors.ErrBadRequest)
			return
		}
		if err := h.svc.ConfigureLDAP(c.Request.Context(), body.Config, body.Enabled, actor); err != nil {
			response.Error(c, errors.ErrInternalServer)
			return
		}
		response.Success(c, http.StatusOK, gin.H{"updated": true})
	default:
		response.Error(c, errors.ErrBadRequest)
	}
}
