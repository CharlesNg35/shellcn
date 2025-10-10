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

type TeamHandler struct {
	svc *services.TeamService
}

type createTeamRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=128"`
	Description string `json:"description" validate:"omitempty,max=512"`
}

type updateTeamRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=128"`
	Description *string `json:"description" validate:"omitempty,max=512"`
}

type teamMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid4"`
}

func NewTeamHandler(db *gorm.DB) (*TeamHandler, error) {
	audit, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}
	svc, err := services.NewTeamService(db, audit)
	if err != nil {
		return nil, err
	}
	return &TeamHandler{svc: svc}, nil
}

// GET /api/teams
func (h *TeamHandler) List(c *gin.Context) {
	teams, err := h.svc.List(requestContext(c))
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, teams)
}

// GET /api/teams/:id
func (h *TeamHandler) Get(c *gin.Context) {
	team, err := h.svc.GetByID(requestContext(c), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, team)
}

// POST /api/teams
func (h *TeamHandler) Create(c *gin.Context) {
	var body createTeamRequest
	if !bindAndValidate(c, &body) {
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		response.Error(c, errors.NewBadRequest("name is required"))
		return
	}

	input := services.CreateTeamInput{
		Name:        name,
		Description: strings.TrimSpace(body.Description),
	}

	team, err := h.svc.Create(requestContext(c), input)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, team)
}

// DELETE /api/teams/:id
func (h *TeamHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(requestContext(c), c.Param("id")); err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

// PATCH /api/teams/:id
func (h *TeamHandler) Update(c *gin.Context) {
	var body updateTeamRequest
	if !bindAndValidate(c, &body) {
		return
	}

	if body.Name == nil && body.Description == nil {
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

	team, err := h.svc.Update(requestContext(c), c.Param("id"), services.UpdateTeamInput{Name: namePtr, Description: descPtr})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, team)
}

// POST /api/teams/:id/members
func (h *TeamHandler) AddMember(c *gin.Context) {
	var body teamMemberRequest
	if !bindAndValidate(c, &body) {
		return
	}
	userID := strings.TrimSpace(body.UserID)
	if userID == "" {
		response.Error(c, errors.NewBadRequest("user id is required"))
		return
	}
	if err := h.svc.AddMember(requestContext(c), c.Param("id"), userID); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"added": true})
}

// DELETE /api/teams/:id/members/:userID
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveMember(requestContext(c), c.Param("id"), c.Param("userID")); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"removed": true})
}

// GET /api/teams/:id/members
func (h *TeamHandler) ListMembers(c *gin.Context) {
	users, err := h.svc.ListMembers(requestContext(c), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, users)
}
