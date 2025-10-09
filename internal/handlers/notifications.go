package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/notifications"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// NotificationHandler exposes HTTP endpoints for notifications.
type NotificationHandler struct {
	service *services.NotificationService
	hub     *notifications.Hub
	jwt     *iauth.JWTService
}

// NewNotificationHandler constructs a notification handler.
func NewNotificationHandler(db *gorm.DB, hub *notifications.Hub, jwt *iauth.JWTService) (*NotificationHandler, error) {
	service, err := services.NewNotificationService(db, hub)
	if err != nil {
		return nil, err
	}
	return &NotificationHandler{
		service: service,
		hub:     hub,
		jwt:     jwt,
	}, nil
}

// List returns notifications for the current user.
func (h *NotificationHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	limit := parseIntQuery(c, "limit", 25)
	offset := parseIntQuery(c, "offset", 0)

	items, err := h.service.ListForUser(c.Request.Context(), services.ListNotificationsInput{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, items)
}

// MarkRead toggles a notification to read.
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	h.updateReadState(c, true)
}

// MarkUnread toggles a notification to unread.
func (h *NotificationHandler) MarkUnread(c *gin.Context) {
	h.updateReadState(c, false)
}

func (h *NotificationHandler) updateReadState(c *gin.Context, read bool) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	var dto *services.NotificationDTO
	var err error
	if read {
		dto, err = h.service.MarkRead(c.Request.Context(), userID, id)
	} else {
		dto, err = h.service.MarkUnread(c.Request.Context(), userID, id)
	}

	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, dto)
}

// Delete removes a notification.
func (h *NotificationHandler) Delete(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if err := h.service.Delete(c.Request.Context(), userID, id); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// MarkAllRead marks all notifications read.
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if err := h.service.MarkAllRead(c.Request.Context(), userID); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// Stream upgrades the connection to a WebSocket for notification streaming.
func (h *NotificationHandler) Stream(c *gin.Context) {
	if h.jwt == nil || h.hub == nil {
		response.Error(c, errors.ErrNotFound)
		return
	}

	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		authz := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token = strings.TrimSpace(authz[7:])
		}
	}

	if token == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	claims, err := h.jwt.ValidateAccessToken(token)
	if err != nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	h.hub.Serve(claims.UserID, c.Writer, c.Request)
}

// Create allows internal systems to create a notification (primarily for tests/admin).
func (h *NotificationHandler) Create(c *gin.Context) {
	var payload struct {
		UserID    string         `json:"user_id"`
		Type      string         `json:"type"`
		Title     string         `json:"title"`
		Message   string         `json:"message"`
		Severity  string         `json:"severity"`
		ActionURL string         `json:"action_url"`
		Metadata  map[string]any `json:"metadata"`
		IsRead    bool           `json:"is_read"`
	}

	if !bindAndValidate(c, &payload) {
		return
	}

	dto, err := h.service.Create(c.Request.Context(), services.CreateNotificationInput{
		UserID:    payload.UserID,
		Type:      payload.Type,
		Title:     payload.Title,
		Message:   payload.Message,
		Severity:  payload.Severity,
		ActionURL: payload.ActionURL,
		Metadata:  payload.Metadata,
		IsRead:    payload.IsRead,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusCreated, dto)
}
