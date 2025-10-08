package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// AuthHandler manages authentication flows (login/refresh/logout/me).
type AuthHandler struct {
	db       *gorm.DB
	jwt      *iauth.JWTService
	sessions *iauth.SessionService
}

func NewAuthHandler(db *gorm.DB, jwt *iauth.JWTService, sessions *iauth.SessionService) *AuthHandler {
	return &AuthHandler{db: db, jwt: jwt, sessions: sessions}
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Identifier) == "" || req.Password == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}

	lp, err := providers.NewLocalProvider(h.db, providers.LocalConfig{})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	user, err := lp.Authenticate(providers.AuthenticateInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		// Normalise auth errors to 401
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	pair, _, err := h.sessions.CreateSession(user.ID, iauth.SessionMetadata{
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	// Include basic user info and permissions in the response
	checker, _ := permissions.NewChecker(h.db)
	perms, _ := checker.GetUserPermissions(c.Request.Context(), user.ID)

	payload := gin.H{
		"tokens": tokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken},
		"user": gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"is_root":    user.IsRoot,
			"is_active":  user.IsActive,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
		},
		"permissions": perms,
	}

	response.Success(c, http.StatusOK, payload)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}

	pair, _, err := h.sessions.RefreshSession(req.RefreshToken)
	if err != nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	response.Success(c, http.StatusOK, tokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	v, ok := c.Get("sessionID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	sid, _ := v.(string)
	if sid == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if err := h.sessions.RevokeSession(sid); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"revoked": true})
}

// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	v, ok := c.Get("userID")
	if !ok {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	userID, _ := v.(string)

	var user models.User
	if err := h.db.Preload("Organization").Preload("Teams").Preload("Roles").Take(&user, "id = ?", userID).Error; err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}

	checker, _ := permissions.NewChecker(h.db)
	perms, _ := checker.GetUserPermissions(c.Request.Context(), user.ID)

	payload := gin.H{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"is_root":     user.IsRoot,
		"is_active":   user.IsActive,
		"first_name":  user.FirstName,
		"last_name":   user.LastName,
		"permissions": perms,
	}

	response.Success(c, http.StatusOK, payload)
}
