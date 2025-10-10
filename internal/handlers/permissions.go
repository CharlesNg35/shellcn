package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type PermissionHandler struct {
	svc *services.PermissionService
}

func NewPermissionHandler(db *gorm.DB) (*PermissionHandler, error) {
	svc, err := services.NewPermissionService(db)
	if err != nil {
		return nil, err
	}
	return &PermissionHandler{svc: svc}, nil
}

// GET /api/permissions/registry
func (h *PermissionHandler) Registry(c *gin.Context) {
	defs := permissions.GetAll()
	response.Success(c, http.StatusOK, defs)
}

// GET /api/permissions/my
func (h *PermissionHandler) MyPermissions(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	perms, err := h.svc.ListUserPermissions(requestContext(c), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, perms)
}

// GET /api/permissions/roles
func (h *PermissionHandler) ListRoles(c *gin.Context) {
	roles, err := h.svc.ListRoles(requestContext(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, roles)
}

// POST /api/permissions/roles
func (h *PermissionHandler) CreateRole(c *gin.Context) {
	var body struct {
		Name, Description string
		IsSystem          bool
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	role, err := h.svc.CreateRole(requestContext(c), services.CreateRoleInput{Name: body.Name, Description: body.Description, IsSystem: body.IsSystem})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusCreated, role)
}

// PATCH /api/permissions/roles/:id
func (h *PermissionHandler) UpdateRole(c *gin.Context) {
	var body struct{ Name, Description string }
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	role, err := h.svc.UpdateRole(requestContext(c), c.Param("id"), services.UpdateRoleInput{Name: body.Name, Description: body.Description})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, role)
}

// DELETE /api/permissions/roles/:id
func (h *PermissionHandler) DeleteRole(c *gin.Context) {
	if err := h.svc.DeleteRole(requestContext(c), c.Param("id")); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// POST /api/permissions/roles/:id/permissions
func (h *PermissionHandler) SetRolePermissions(c *gin.Context) {
	var body struct{ Permissions []string }
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	if err := h.svc.SetRolePermissions(requestContext(c), c.Param("id"), body.Permissions); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"updated": true})
}
