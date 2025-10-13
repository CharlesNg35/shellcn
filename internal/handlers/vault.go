package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// VaultHandler exposes vault related APIs.
type VaultHandler struct {
	service *services.VaultService
}

// NewVaultHandler constructs a handler for the vault service.
func NewVaultHandler(svc *services.VaultService) *VaultHandler {
	return &VaultHandler{service: svc}
}

type createIdentityPayload struct {
	Name         string         `json:"name" validate:"required,min=2,max=128"`
	Description  string         `json:"description" validate:"omitempty,max=512"`
	Scope        string         `json:"scope" validate:"required,oneof=global team connection"`
	TemplateID   *string        `json:"template_id" validate:"omitempty,uuid4"`
	TeamID       *string        `json:"team_id" validate:"omitempty,uuid4"`
	ConnectionID *string        `json:"connection_id" validate:"omitempty,uuid4"`
	Metadata     map[string]any `json:"metadata"`
	Payload      map[string]any `json:"payload" validate:"required"`
	OwnerUserID  *string        `json:"owner_user_id" validate:"omitempty,uuid4"`
}

type updateIdentityPayload struct {
	Name         *string        `json:"name" validate:"omitempty,min=2,max=128"`
	Description  *string        `json:"description" validate:"omitempty,max=512"`
	Metadata     map[string]any `json:"metadata"`
	Payload      map[string]any `json:"payload"`
	TemplateID   *string        `json:"template_id" validate:"omitempty,uuid4"`
	ConnectionID *string        `json:"connection_id" validate:"omitempty,uuid4"`
}

type createSharePayload struct {
	PrincipalType string         `json:"principal_type" validate:"required,oneof=user team"`
	PrincipalID   string         `json:"principal_id" validate:"required"`
	Permission    string         `json:"permission" validate:"required,oneof=use view_metadata edit"`
	ExpiresAt     *time.Time     `json:"expires_at"`
	Metadata      map[string]any `json:"metadata"`
}

// ListIdentities returns identities visible to the authenticated user.
func (h *VaultHandler) ListIdentities(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	opts := services.ListIdentitiesOptions{}
	scope := strings.TrimSpace(c.Query("scope"))
	if scope != "" {
		opts.Scope = models.IdentityScope(scope)
	}
	opts.ProtocolID = strings.TrimSpace(c.Query("protocol_id"))
	opts.IncludeConnectionScoped = parseBool(c.Query("include_connection_scoped"))

	identities, err := h.service.ListIdentities(ctx, viewer, opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	if identities == nil {
		identities = []services.IdentityDTO{}
	}
	response.Success(c, http.StatusOK, identities)
}

// CreateIdentity stores a new credential identity.
func (h *VaultHandler) CreateIdentity(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var payload createIdentityPayload
	if !bindAndValidate(c, &payload) {
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	scope := models.IdentityScope(strings.TrimSpace(payload.Scope))
	ownerID := userID
	if payload.OwnerUserID != nil && strings.TrimSpace(*payload.OwnerUserID) != "" {
		ownerID = strings.TrimSpace(*payload.OwnerUserID)
	}

	identity, err := h.service.CreateIdentity(ctx, viewer, services.CreateIdentityInput{
		Name:         payload.Name,
		Description:  payload.Description,
		Scope:        scope,
		TemplateID:   payload.TemplateID,
		TeamID:       payload.TeamID,
		ConnectionID: payload.ConnectionID,
		Metadata:     payload.Metadata,
		Payload:      payload.Payload,
		OwnerUserID:  ownerID,
		CreatedBy:    userID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusCreated, identity)
}

// GetIdentity retrieves a single identity.
func (h *VaultHandler) GetIdentity(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	includePayload := strings.Contains(strings.ToLower(c.Query("include")), "payload")

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	identity, err := h.service.GetIdentity(ctx, viewer, c.Param("id"), includePayload)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, identity)
}

// UpdateIdentity mutates an existing identity.
func (h *VaultHandler) UpdateIdentity(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var payload updateIdentityPayload
	if !bindAndValidate(c, &payload) {
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	identity, err := h.service.UpdateIdentity(ctx, viewer, c.Param("id"), services.UpdateIdentityInput{
		Name:         payload.Name,
		Description:  payload.Description,
		Metadata:     payload.Metadata,
		Payload:      payload.Payload,
		TemplateID:   payload.TemplateID,
		ConnectionID: payload.ConnectionID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, identity)
}

// DeleteIdentity removes an identity.
func (h *VaultHandler) DeleteIdentity(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.DeleteIdentity(ctx, viewer, c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// CreateShare grants an identity share to a principal.
func (h *VaultHandler) CreateShare(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	var payload createSharePayload
	if !bindAndValidate(c, &payload) {
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	share, err := h.service.CreateShare(ctx, viewer, c.Param("id"), services.IdentityShareInput{
		PrincipalType: models.IdentitySharePrincipal(payload.PrincipalType),
		PrincipalID:   payload.PrincipalID,
		Permission:    models.IdentitySharePermission(payload.Permission),
		ExpiresAt:     payload.ExpiresAt,
		Metadata:      payload.Metadata,
		CreatedBy:     userID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusCreated, share)
}

// DeleteShare revokes a share by identifier.
func (h *VaultHandler) DeleteShare(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, appErrors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	viewer, err := h.service.ResolveViewer(ctx, userID, isRootRequest(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	if err := h.service.DeleteShare(ctx, viewer, c.Param("shareId")); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListTemplates returns all credential templates.
func (h *VaultHandler) ListTemplates(c *gin.Context) {
	ctx := requestContext(c)
	templates, err := h.service.ListTemplates(ctx)
	if err != nil {
		response.Error(c, err)
		return
	}
	if templates == nil {
		templates = []services.TemplateDTO{}
	}
	response.Success(c, http.StatusOK, templates)
}

func isRootRequest(c *gin.Context) bool {
	value, exists := c.Get(middleware.CtxClaimsKey)
	if !exists || value == nil {
		return false
	}
	claims, ok := value.(*iauth.Claims)
	if !ok || claims.Metadata == nil {
		return false
	}
	if flag, ok := claims.Metadata["is_root"].(bool); ok {
		return flag
	}
	if flag, ok := claims.Metadata["is_root"].(float64); ok {
		return flag != 0
	}
	return false
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
