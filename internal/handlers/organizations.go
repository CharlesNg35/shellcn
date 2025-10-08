package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type OrganizationHandler struct {
	svc *services.OrganizationService
}

func NewOrganizationHandler(db *gorm.DB) (*OrganizationHandler, error) {
	audit, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}
	svc, err := services.NewOrganizationService(db, audit)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{svc: svc}, nil
}

// GET /api/orgs
func (h *OrganizationHandler) List(c *gin.Context) {
	orgs, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, orgs)
}

// GET /api/orgs/:id
func (h *OrganizationHandler) Get(c *gin.Context) {
	org, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, org)
}

// POST /api/orgs
func (h *OrganizationHandler) Create(c *gin.Context) {
	var body struct {
		Name, Description string
		Settings          map[string]any
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	org, err := h.svc.Create(c.Request.Context(), services.CreateOrganizationInput{Name: body.Name, Description: body.Description, Settings: body.Settings})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, org)
}

// PATCH /api/orgs/:id
func (h *OrganizationHandler) Update(c *gin.Context) {
	var body struct {
		Name        *string
		Description *string
		Settings    map[string]any
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	org, err := h.svc.Update(c.Request.Context(), c.Param("id"), services.UpdateOrganizationInput{Name: body.Name, Description: body.Description, Settings: body.Settings})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, org)
}

// DELETE /api/orgs/:id
func (h *OrganizationHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}
