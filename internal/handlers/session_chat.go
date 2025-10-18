package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/auditctx"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// SessionChatHandler exposes endpoints for posting and listing session chat messages.
type SessionChatHandler struct {
	chats     *services.SessionChatService
	lifecycle *services.SessionLifecycleService
}

// NewSessionChatHandler constructs a chat handler when dependencies are provided.
func NewSessionChatHandler(chatSvc *services.SessionChatService, lifecycle *services.SessionLifecycleService) *SessionChatHandler {
	return &SessionChatHandler{
		chats:     chatSvc,
		lifecycle: lifecycle,
	}
}

type chatMessageRequest struct {
	Content string `json:"content"`
}

// PostMessage persists a chat message for the specified session.
func (h *SessionChatHandler) PostMessage(c *gin.Context) {
	if h == nil || h.chats == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	var payload chatMessageRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid chat message payload"))
		return
	}
	payload.Content = strings.TrimSpace(payload.Content)
	if payload.Content == "" {
		response.Error(c, apperrors.NewBadRequest("message content is required"))
		return
	}

	session, err := h.lifecycle.AuthorizeSessionAccess(c.Request.Context(), sessionID, userID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session"))
		}
		return
	}

	if session.ClosedAt != nil {
		response.Error(c, apperrors.NewBadRequest("session is no longer active"))
		return
	}

	actor, _ := auditctx.FromContext(c.Request.Context())
	message, err := h.chats.PostMessage(c.Request.Context(), services.ChatMessageParams{
		SessionID: sessionID,
		AuthorID:  userID,
		Author:    actor.Username,
		Content:   payload.Content,
	})
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "unable to post chat message"))
		return
	}

	response.Success(c, http.StatusCreated, toChatMessageDTO(message.MessageID, message.SessionID, message.AuthorID, message.Content, message.CreatedAt))
}

// ListMessages returns recent chat history for a session.
func (h *SessionChatHandler) ListMessages(c *gin.Context) {
	if h == nil || h.chats == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	if _, err := h.lifecycle.AuthorizeSessionAccess(c.Request.Context(), sessionID, userID); err != nil {
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			response.Error(c, apperrors.ErrNotFound)
		case errors.Is(err, services.ErrSessionAccessDenied):
			response.Error(c, apperrors.ErrForbidden)
		default:
			response.Error(c, apperrors.Wrap(err, "failed to authorise session"))
		}
		return
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.DefaultQuery("limit", "")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			if parsed > 200 {
				limit = 200
			} else {
				limit = parsed
			}
		}
	}

	var before time.Time
	if raw := strings.TrimSpace(c.Query("before")); raw != "" {
		if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			before = ts
		}
	}

	messages, err := h.chats.ListMessages(c.Request.Context(), sessionID, limit, before)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "failed to list chat messages"))
		return
	}

	dtos := make([]chatMessageDTO, 0, len(messages))
	for _, msg := range messages {
		dtos = append(dtos, toChatMessageDTO(msg.ID, msg.SessionID, msg.AuthorID, msg.Content, msg.CreatedAt))
	}

	response.Success(c, http.StatusOK, dtos)
}

type chatMessageDTO struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	AuthorID  string    `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func toChatMessageDTO(id, sessionID, authorID, content string, createdAt time.Time) chatMessageDTO {
	return chatMessageDTO{
		ID:        id,
		SessionID: sessionID,
		AuthorID:  authorID,
		Content:   content,
		CreatedAt: createdAt,
	}
}
