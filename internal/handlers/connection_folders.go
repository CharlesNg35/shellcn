package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ConnectionFolderHandler exposes HTTP endpoints for managing folders.
type ConnectionFolderHandler struct {
	svc *services.ConnectionFolderService
}

// NewConnectionFolderHandler constructs a folder handler.
func NewConnectionFolderHandler(svc *services.ConnectionFolderService) *ConnectionFolderHandler {
	return &ConnectionFolderHandler{svc: svc}
}

// ListTree returns the folder hierarchy for the current user.
func (h *ConnectionFolderHandler) ListTree(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	teamQuery := strings.TrimSpace(c.Query("team_id"))
	var teamFilter *string
	if teamQuery != "" {
		teamFilter = &teamQuery
	}

	tree, err := h.svc.ListTree(requestContext(c), userID, teamFilter)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, tree)
}

// Create registers a new connection folder.
func (h *ConnectionFolderHandler) Create(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var payload folderPayload
	if !bindAndValidate(c, &payload) {
		return
	}

	dto, err := h.svc.Create(requestContext(c), userID, payload.toInput())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusCreated, dto)
}

// Update updates folder metadata.
func (h *ConnectionFolderHandler) Update(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var payload folderPayload
	if !bindAndValidate(c, &payload) {
		return
	}

	dto, err := h.svc.Update(requestContext(c), userID, c.Param("id"), payload.toInput())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, dto)
}

// Delete removes a folder.
func (h *ConnectionFolderHandler) Delete(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if err := h.svc.Delete(requestContext(c), userID, c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

type folderPayload struct {
	Name        string         `json:"name" binding:"omitempty"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Color       string         `json:"color"`
	ParentID    *string        `json:"parent_id"`
	TeamID      *string        `json:"team_id"`
	Metadata    map[string]any `json:"metadata"`
	Ordering    *int           `json:"ordering"`
}

func (p folderPayload) toInput() services.ConnectionFolderInput {
	return services.ConnectionFolderInput{
		Name:        p.Name,
		Description: p.Description,
		Icon:        p.Icon,
		Color:       p.Color,
		ParentID:    p.ParentID,
		TeamID:      p.TeamID,
		Metadata:    p.Metadata,
		Ordering:    p.Ordering,
	}
}
