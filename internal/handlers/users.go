package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type UserHandler struct {
	service *services.UserService
}

type createUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=64"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	IsRoot   bool   `json:"is_root"`
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
	var body createUserRequest
	if !bindAndValidate(c, &body) {
		return
	}

	input := services.CreateUserInput{
		Username: strings.TrimSpace(body.Username),
		Email:    strings.ToLower(strings.TrimSpace(body.Email)),
		Password: body.Password,
		IsRoot:   body.IsRoot,
	}

	if input.Username == "" {
		response.Error(c, errors.NewBadRequest("username is required"))
		return
	}
	if input.Email == "" {
		response.Error(c, errors.NewBadRequest("email is required"))
		return
	}

	user, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, user)
}
