package handlers

import (
	"net/http"
	"strings"

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

type createOrganizationRequest struct {
	Name        string         `json:"name" validate:"required,min=3,max=128"`
	Description string         `json:"description" validate:"omitempty,max=512"`
	Settings    map[string]any `json:"settings"`
}

type updateOrganizationRequest struct {
	Name        *string         `json:"name" validate:"omitempty,min=3,max=128"`
	Description *string         `json:"description" validate:"omitempty,max=512"`
	Settings    *map[string]any `json:"settings"`
}

// GET /api/orgs
func (h *OrganizationHandler) List(c *gin.Context) {
	orgs, err := h.svc.List(requestContext(c))
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, orgs)
}

// GET /api/orgs/:id
func (h *OrganizationHandler) Get(c *gin.Context) {
	org, err := h.svc.GetByID(requestContext(c), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, org)
}

// POST /api/orgs
func (h *OrganizationHandler) Create(c *gin.Context) {
	var body createOrganizationRequest
	if !bindAndValidate(c, &body) {
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		response.Error(c, errors.NewBadRequest("name is required"))
		return
	}

	input := services.CreateOrganizationInput{
		Name:        name,
		Description: strings.TrimSpace(body.Description),
		Settings:    body.Settings,
	}

	org, err := h.svc.Create(requestContext(c), input)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, org)
}

// PATCH /api/orgs/:id
func (h *OrganizationHandler) Update(c *gin.Context) {
	var body updateOrganizationRequest
	if !bindAndValidate(c, &body) {
		return
	}

	if body.Name == nil && body.Description == nil && body.Settings == nil {
		response.Error(c, errors.NewBadRequest("no fields provided for update"))
		return
	}

	var namePtr *string
	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			response.Error(c, errors.NewBadRequest("name must not be empty"))
			return
		}
		namePtr = &trimmed
	}

	var descPtr *string
	if body.Description != nil {
		trimmed := strings.TrimSpace(*body.Description)
		descPtr = &trimmed
	}

	var settings map[string]any
	if body.Settings != nil {
		settings = *body.Settings
	}

	org, err := h.svc.Update(requestContext(c), c.Param("id"), services.UpdateOrganizationInput{Name: namePtr, Description: descPtr, Settings: settings})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, org)
}

// DELETE /api/orgs/:id
func (h *OrganizationHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(requestContext(c), c.Param("id")); err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}
