package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type SessionHandler struct {
	db       *gorm.DB
	sessions *iauth.SessionService
}

func NewSessionHandler(db *gorm.DB, sessions *iauth.SessionService) *SessionHandler {
	return &SessionHandler{db: db, sessions: sessions}
}

// GET /api/sessions/me
func (h *SessionHandler) ListMySessions(c *gin.Context) {
	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	userID := v.(string)

	var sessions []models.Session
	if err := h.db.Where("user_id = ?", userID).Order("last_used_at DESC").Find(&sessions).Error; err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, sessions)
}

// POST /api/sessions/revoke/:id
func (h *SessionHandler) Revoke(c *gin.Context) {
	if err := h.sessions.RevokeSession(c.Param("id")); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"revoked": true})
}

// POST /api/sessions/revoke_all
func (h *SessionHandler) RevokeAll(c *gin.Context) {
	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	userID := v.(string)

	if err := h.sessions.RevokeUserSessions(userID); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"revoked": true})
}
