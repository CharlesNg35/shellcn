package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type TeamHandler struct {
	svc         *services.TeamService
	connections *services.ConnectionService
	folderSvc   *services.ConnectionFolderService
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

type updateTeamRolesRequest struct {
	RoleIDs []string `json:"role_ids" validate:"omitempty,dive,required"`
}

func NewTeamHandler(
	db *gorm.DB,
	checker *permissions.Checker,
	connectionSvc *services.ConnectionService,
	folderSvc *services.ConnectionFolderService,
) (*TeamHandler, error) {
	audit, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}

	if checker == nil {
		checker, err = permissions.NewChecker(db)
		if err != nil {
			return nil, err
		}
	}

	svc, err := services.NewTeamService(db, audit, checker)
	if err != nil {
		return nil, err
	}

	if connectionSvc == nil {
		connectionSvc, err = services.NewConnectionService(db, checker)
		if err != nil {
			return nil, err
		}
	}

	if folderSvc == nil {
		folderSvc, err = services.NewConnectionFolderService(db, checker, connectionSvc)
		if err != nil {
			return nil, err
		}
	}

	return &TeamHandler{svc: svc, connections: connectionSvc, folderSvc: folderSvc}, nil
}

// GET /api/teams
func (h *TeamHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	teams, err := h.svc.List(requestContext(c), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, teams)
}

// GET /api/teams/:id
func (h *TeamHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	team, err := h.svc.GetByID(requestContext(c), c.Param("id"), userID)
	if err != nil {
		response.Error(c, err)
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
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusCreated, team)
}

// DELETE /api/teams/:id
func (h *TeamHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(requestContext(c), c.Param("id")); err != nil {
		response.Error(c, err)
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
		response.Error(c, err)
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
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"added": true})
}

// DELETE /api/teams/:id/members/:userID
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveMember(requestContext(c), c.Param("id"), c.Param("userID")); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"removed": true})
}

// GET /api/teams/:id/members
func (h *TeamHandler) ListMembers(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	users, err := h.svc.ListMembers(requestContext(c), userID, c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, users)
}

// GET /api/teams/:id/roles
func (h *TeamHandler) ListRoles(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}
	roles, err := h.svc.ListRoles(requestContext(c), userID, c.Param("id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, roles)
}

// GET /api/teams/:id/connections
func (h *TeamHandler) ListConnections(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	teamID := strings.TrimSpace(c.Param("id"))
	ctx := requestContext(c)
	if _, err := h.svc.GetByID(ctx, teamID, userID); err != nil {
		response.Error(c, err)
		return
	}

	includeTargets, includeVisibility := parseConnectionIncludes(c.Query("include"))
	page := parseIntQuery(c, "page", 1)
	perPage := parseIntQuery(c, "per_page", 25)

	result, err := h.connections.ListVisible(ctx, services.ListConnectionsOptions{
		UserID:            userID,
		TeamID:            teamID,
		FolderID:          c.Query("folder_id"),
		IncludeTargets:    includeTargets,
		IncludeVisibility: includeVisibility,
		Page:              page,
		PerPage:           perPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	meta := &response.Meta{
		Page:       result.Page,
		PerPage:    result.PerPage,
		Total:      int(result.Total),
		TotalPages: computePages(result.Total, int64(result.PerPage)),
	}
	response.SuccessWithMeta(c, http.StatusOK, result.Connections, meta)
}

// GET /api/teams/:id/folders
func (h *TeamHandler) ListConnectionFolders(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	teamID := strings.TrimSpace(c.Param("id"))
	ctx := requestContext(c)
	if _, err := h.svc.GetByID(ctx, teamID, userID); err != nil {
		response.Error(c, err)
		return
	}

	teamParam := teamID
	nodes, err := h.folderSvc.ListTree(ctx, userID, &teamParam)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, nodes)
}

// PUT /api/teams/:id/roles
func (h *TeamHandler) SetRoles(c *gin.Context) {
	var body updateTeamRolesRequest
	if !bindAndValidate(c, &body) {
		return
	}

	roles, err := h.svc.SetRoles(requestContext(c), c.Param("id"), body.RoleIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, roles)
}

func parseConnectionIncludes(includeParam string) (bool, bool) {
	includeTargets := false
	includeVisibility := false
	if includeParam == "" {
		return true, false
	}

	for _, part := range strings.Split(includeParam, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "targets":
			includeTargets = true
		case "visibility":
			includeVisibility = true
		}
	}
	return includeTargets, includeVisibility
}

func computePages(total, perPage int64) int {
	if perPage <= 0 {
		return 1
	}
	pages := total / perPage
	if total%perPage != 0 {
		pages++
	}
	if pages == 0 {
		return 1
	}
	return int(pages)
}
