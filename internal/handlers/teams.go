package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type TeamHandler struct {
	svc *services.TeamService
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

// GET /api/orgs/:orgID/teams
func (h *TeamHandler) ListByOrg(c *gin.Context) {
	teams, err := h.svc.ListByOrganization(c.Request.Context(), c.Param("orgID"))
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, teams)
}

// GET /api/teams/:id
func (h *TeamHandler) Get(c *gin.Context) {
	team, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrNotFound)
		return
	}
	response.Success(c, http.StatusOK, team)
}

// POST /api/teams
func (h *TeamHandler) Create(c *gin.Context) {
	var body struct{ OrganizationID, Name, Description string }
	if err := c.ShouldBindJSON(&body); err != nil || body.OrganizationID == "" || body.Name == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	team, err := h.svc.Create(c.Request.Context(), services.CreateTeamInput{OrganizationID: body.OrganizationID, Name: body.Name, Description: body.Description})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusCreated, team)
}

// PATCH /api/teams/:id
func (h *TeamHandler) Update(c *gin.Context) {
	var body struct {
		Name        *string
		Description *string
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	team, err := h.svc.Update(c.Request.Context(), c.Param("id"), services.UpdateTeamInput{Name: body.Name, Description: body.Description})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, team)
}

// POST /api/teams/:id/members
func (h *TeamHandler) AddMember(c *gin.Context) {
	var body struct{ UserID string }
	if err := c.ShouldBindJSON(&body); err != nil || body.UserID == "" {
		response.Error(c, errors.ErrBadRequest)
		return
	}
	if err := h.svc.AddMember(c.Request.Context(), c.Param("id"), body.UserID); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"added": true})
}

// DELETE /api/teams/:id/members/:userID
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveMember(c.Request.Context(), c.Param("id"), c.Param("userID")); err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"removed": true})
}

// GET /api/teams/:id/members
func (h *TeamHandler) ListMembers(c *gin.Context) {
	users, err := h.svc.ListMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, users)
}
