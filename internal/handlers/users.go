package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type UserHandler struct {
	service *services.UserService
}

type createUserRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=64"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"omitempty,max=128"`
	LastName  string `json:"last_name" validate:"omitempty,max=128"`
	Avatar    string `json:"avatar" validate:"omitempty,max=512"`
	IsRoot    bool   `json:"is_root"`
	IsActive  *bool  `json:"is_active"`
}

type updateUserRequest struct {
	Username  *string `json:"username" validate:"omitempty,min=3,max=64"`
	Email     *string `json:"email" validate:"omitempty,email"`
	FirstName *string `json:"first_name" validate:"omitempty,max=128"`
	LastName  *string `json:"last_name" validate:"omitempty,max=128"`
	Avatar    *string `json:"avatar" validate:"omitempty,max=512"`
}

type userPasswordRequest struct {
	Password string `json:"password" validate:"required,min=8"`
}

type bulkUserRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1,dive,uuid4"`
}

type bulkActivationRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1,dive,uuid4"`
	Active  *bool    `json:"active"`
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
	page := parseIntQuery(c, "page", 1)
	perPage := parseIntQuery(c, "per_page", 20)
	if perPage <= 0 {
		perPage = 20
	}

	search := strings.TrimSpace(c.DefaultQuery("search", c.DefaultQuery("query", "")))
	statusFilter := strings.ToLower(strings.TrimSpace(c.Query("status")))

	var isActive *bool
	switch statusFilter {
	case "active":
		active := true
		isActive = &active
	case "inactive":
		active := false
		isActive = &active
	}

	filters := services.UserFilters{
		Query: search,
	}
	if isActive != nil {
		filters.IsActive = isActive
	}

	users, total, err := h.service.List(requestContext(c), services.ListUsersOptions{
		Page:     page,
		PageSize: perPage,
		Filters:  filters,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	totalPages := 0
	if perPage > 0 {
		totalPages = int((total + int64(perPage) - 1) / int64(perPage))
	}

	response.SuccessWithMeta(c, http.StatusOK, users, &response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      int(total),
		TotalPages: totalPages,
	})
}

// GET /api/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id := c.Param("id")
	user, err := h.service.GetByID(requestContext(c), id)
	if err != nil {
		respondUserError(c, err, "retrieved")
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

	username := strings.TrimSpace(body.Username)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if username == "" {
		response.Error(c, appErrors.NewBadRequest("username is required"))
		return
	}
	if email == "" {
		response.Error(c, appErrors.NewBadRequest("email is required"))
		return
	}

	input := services.CreateUserInput{
		Username:  username,
		Email:     email,
		Password:  body.Password,
		FirstName: strings.TrimSpace(body.FirstName),
		LastName:  strings.TrimSpace(body.LastName),
		Avatar:    strings.TrimSpace(body.Avatar),
		IsRoot:    body.IsRoot,
		IsActive:  body.IsActive,
	}

	user, err := h.service.Create(requestContext(c), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusCreated, user)
}

// PATCH /api/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	var body updateUserRequest
	if !bindAndValidate(c, &body) {
		return
	}

	if body.Username == nil && body.Email == nil && body.FirstName == nil &&
		body.LastName == nil && body.Avatar == nil {
		response.Error(c, appErrors.NewBadRequest("no fields provided for update"))
		return
	}

	var usernamePtr *string
	if body.Username != nil {
		trimmed := strings.TrimSpace(*body.Username)
		if trimmed == "" {
			response.Error(c, appErrors.NewBadRequest("username must not be empty"))
			return
		}
		username := trimmed
		usernamePtr = &username
	}

	var emailPtr *string
	if body.Email != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*body.Email))
		if trimmed == "" {
			response.Error(c, appErrors.NewBadRequest("email must not be empty"))
			return
		}
		email := trimmed
		emailPtr = &email
	}

	var firstPtr *string
	if body.FirstName != nil {
		first := strings.TrimSpace(*body.FirstName)
		firstPtr = &first
	}

	var lastPtr *string
	if body.LastName != nil {
		last := strings.TrimSpace(*body.LastName)
		lastPtr = &last
	}

	var avatarPtr *string
	if body.Avatar != nil {
		avatar := strings.TrimSpace(*body.Avatar)
		avatarPtr = &avatar
	}

	input := services.UpdateUserInput{
		Username:  usernamePtr,
		Email:     emailPtr,
		FirstName: firstPtr,
		LastName:  lastPtr,
		Avatar:    avatarPtr,
	}

	user, err := h.service.Update(requestContext(c), c.Param("id"), input)
	if err != nil {
		respondUserError(c, err, "updated")
		return
	}

	response.Success(c, http.StatusOK, user)
}

// DELETE /api/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(requestContext(c), c.Param("id")); err != nil {
		respondUserError(c, err, "deleted")
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// POST /api/users/:id/activate
func (h *UserHandler) Activate(c *gin.Context) {
	h.toggleActive(c, true)
}

// POST /api/users/:id/deactivate
func (h *UserHandler) Deactivate(c *gin.Context) {
	h.toggleActive(c, false)
}

func (h *UserHandler) toggleActive(c *gin.Context, active bool) {
	if err := h.service.SetActive(requestContext(c), c.Param("id"), active); err != nil {
		action := "deactivated"
		if active {
			action = "activated"
		}
		respondUserError(c, err, action)
		return
	}

	user, err := h.service.GetByID(requestContext(c), c.Param("id"))
	if err != nil {
		respondUserError(c, err, "retrieved")
		return
	}

	response.Success(c, http.StatusOK, user)
}

// POST /api/users/:id/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var body userPasswordRequest
	if !bindAndValidate(c, &body) {
		return
	}

	if err := h.service.ChangePassword(requestContext(c), c.Param("id"), body.Password); err != nil {
		respondUserError(c, err, "updated password")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"updated": true})
}

// POST /api/users/bulk/activate or deactivate
func (h *UserHandler) BulkActivate(c *gin.Context) {
	h.handleBulkActivation(c, true)
}

func (h *UserHandler) BulkDeactivate(c *gin.Context) {
	h.handleBulkActivation(c, false)
}

func (h *UserHandler) handleBulkActivation(c *gin.Context, defaultActive bool) {
	var body bulkActivationRequest
	if !bindAndValidate(c, &body) {
		return
	}

	if len(body.UserIDs) == 0 {
		response.Error(c, appErrors.NewBadRequest("user_ids is required"))
		return
	}

	active := defaultActive
	if body.Active != nil {
		active = *body.Active
	}

	ctx := requestContext(c)
	successCount := 0
	failures := make(map[string]string)

	for _, id := range body.UserIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := h.service.SetActive(ctx, id, active); err != nil {
			failures[id] = classifyUserError(err)
			continue
		}
		successCount++
	}

	response.Success(c, http.StatusOK, gin.H{
		"updated":   successCount,
		"failed":    failures,
		"is_active": active,
	})
}

// DELETE /api/users/bulk
func (h *UserHandler) BulkDelete(c *gin.Context) {
	var body bulkUserRequest
	if !bindAndValidate(c, &body) {
		return
	}
	if len(body.UserIDs) == 0 {
		response.Error(c, appErrors.NewBadRequest("user_ids is required"))
		return
	}

	ctx := requestContext(c)
	successCount := 0
	failures := make(map[string]string)

	for _, id := range body.UserIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := h.service.Delete(ctx, id); err != nil {
			failures[id] = classifyUserError(err)
			continue
		}
		successCount++
	}

	response.Success(c, http.StatusOK, gin.H{
		"deleted": successCount,
		"failed":  failures,
	})
}

func respondUserError(c *gin.Context, err error, action string) {
	if err == nil {
		response.Error(c, appErrors.ErrInternalServer)
		return
	}

	if errors.Is(err, services.ErrUserNotFound) {
		response.Error(c, appErrors.ErrNotFound)
		return
	}

	if errors.Is(err, services.ErrRootUserImmutable) {
		if action == "" {
			action = "modified"
		}
		response.Error(c, appErrors.NewBadRequest("root user cannot be "+action))
		return
	}

	if appErr, ok := err.(*appErrors.AppError); ok {
		response.Error(c, appErr)
		return
	}

	response.Error(c, appErrors.ErrInternalServer)
}

func classifyUserError(err error) string {
	if errors.Is(err, services.ErrUserNotFound) {
		return "not_found"
	}
	if errors.Is(err, services.ErrRootUserImmutable) {
		return "root_user_immutable"
	}
	if appErr, ok := err.(*appErrors.AppError); ok {
		if appErr.StatusCode >= 400 && appErr.StatusCode < 500 {
			return strings.ToLower(appErr.Code)
		}
	}
	return "internal_error"
}
