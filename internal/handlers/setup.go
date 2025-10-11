package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	appErr "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type SetupHandler struct {
	db    *gorm.DB
	users *services.UserService
}

func NewSetupHandler(db *gorm.DB) (*SetupHandler, error) {
	if db == nil {
		return nil, appErr.New("SETUP_HANDLER_DB_REQUIRED", "setup handler: db is required", http.StatusInternalServerError)
	}
	audit, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}
	users, err := services.NewUserService(db, audit)
	if err != nil {
		return nil, err
	}
	return &SetupHandler{db: db, users: users}, nil
}

// GET /api/setup/status
func (h *SetupHandler) Status(c *gin.Context) {
	var first models.User
	if err := h.db.Order("created_at ASC").Limit(1).Take(&first).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Success(c, http.StatusOK, gin.H{
				"status":      "pending",
				"initialized": false,
				"message":     "Initial setup required",
			})
			return
		}
		response.Error(c, appErr.ErrInternalServer)
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"status":        "complete",
		"initialized":   true,
		"message":       "System is configured",
		"first_user_id": first.ID,
	})
}

// POST /api/setup/initialize
func (h *SetupHandler) Initialize(c *gin.Context) {
	var body struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Email == "" || body.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "BAD_REQUEST", "message": "invalid payload"}})
		return
	}

	// Prevent re-initialization
	var count int64
	if err := h.db.Model(&models.User{}).Count(&count).Error; err == nil && count > 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{"code": "ALREADY_INITIALIZED", "message": "system already initialized"}})
		return
	}

	user, err := h.users.Create(requestContext(c), services.CreateUserInput{
		Username:  body.Username,
		Email:     body.Email,
		Password:  body.Password,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		IsRoot:    true,
		IsActive:  ptrBool(true),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "INTERNAL_SERVER_ERROR", "message": "failed to create root user"}})
		return
	}

	response.Success(c, http.StatusCreated, gin.H{"root_user_id": user.ID})
}

func ptrBool(v bool) *bool { return &v }
