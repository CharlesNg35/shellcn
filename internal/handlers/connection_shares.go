package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ConnectionShareHandler exposes CRUD operations for connection shares.
type ConnectionShareHandler struct {
	svc *services.ConnectionShareService
}

// NewConnectionShareHandler constructs a handler for share endpoints.
func NewConnectionShareHandler(svc *services.ConnectionShareService) *ConnectionShareHandler {
	return &ConnectionShareHandler{svc: svc}
}

// GET /api/connections/:id/shares
func (h *ConnectionShareHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	shares, err := h.svc.ListShares(requestContext(c), userID, c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, shares)
}

type createShareRequest struct {
	UserID           *string        `json:"user_id"`
	TeamID           *string        `json:"team_id"`
	PermissionScopes []string       `json:"permission_scopes" binding:"required,min=1,dive,required"`
	ExpiresAt        *string        `json:"expires_at"`
	Metadata         map[string]any `json:"metadata"`
}

// POST /api/connections/:id/shares
func (h *ConnectionShareHandler) Create(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var body createShareRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}

	var expiresAt *time.Time
	if body.ExpiresAt != nil && strings.TrimSpace(*body.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.ExpiresAt))
		if err != nil {
			response.Error(c, errors.NewBadRequest("expires_at must be RFC3339 timestamp"))
			return
		}
		expiresAt = &t
	}

	principalType := services.PrincipalTypeUser
	principalID := ""
	if body.UserID != nil && strings.TrimSpace(*body.UserID) != "" {
		principalID = strings.TrimSpace(*body.UserID)
	} else if body.TeamID != nil && strings.TrimSpace(*body.TeamID) != "" {
		principalType = services.PrincipalTypeTeam
		principalID = strings.TrimSpace(*body.TeamID)
	} else {
		response.Error(c, errors.NewBadRequest("user_id or team_id is required"))
		return
	}

	result, err := h.svc.CreateShare(requestContext(c), userID, c.Param("id"), services.CreateShareInput{
		PrincipalType: principalType,
		PrincipalID:   principalID,
		PermissionIDs: body.PermissionScopes,
		ExpiresAt:     expiresAt,
		Metadata:      body.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusCreated, result)
}

// DELETE /api/connections/:id/shares/:shareId
func (h *ConnectionShareHandler) Delete(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if err := h.svc.DeleteShare(requestContext(c), userID, c.Param("id"), c.Param("shareId")); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}
