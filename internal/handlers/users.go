package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(db *gorm.DB) (*UserHandler, error) {
	audit, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}
	us, err := services.NewUserService(db, audit)
	if err != nil {
		return nil, err
	}
	return &UserHandler{service: us}, nil
}

// GET /api/users
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	per, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	users, total, err := h.service.List(c.Request.Context(), services.ListUsersOptions{Page: page, PageSize: per})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.SuccessWithMeta(c, http.StatusOK, users, &response.Meta{Page: page, PerPage: per, Total: int(total)})
}

// GET /api/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id := c.Param("id")
	user, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, user)
}

// POST /api/users
func (h *UserHandler) Create(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		IsRoot   bool   `json:"is_root"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Username == "" || body.Email == "" || body.Password == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}

	user, err := h.service.Create(c.Request.Context(), services.CreateUserInput{Username: body.Username, Email: body.Email, Password: body.Password, IsRoot: body.IsRoot})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, user)
}
