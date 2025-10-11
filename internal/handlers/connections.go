package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ConnectionHandler exposes connection APIs.
type ConnectionHandler struct {
	svc      *services.ConnectionService
	shareSvc *services.ConnectionShareService
}

// NewConnectionHandler constructs a handler using the provided service.
func NewConnectionHandler(svc *services.ConnectionService, shareSvc *services.ConnectionShareService) *ConnectionHandler {
	return &ConnectionHandler{svc: svc, shareSvc: shareSvc}
}

// List returns visible connections for the authenticated user.
func (h *ConnectionHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	includeTargets, includeGrants := parseIncludes(c.Query("include"))
	page := parseIntQuery(c, "page", 1)
	perPage := parseIntQuery(c, "per_page", 25)

	ctx := requestContext(c)
	result, err := h.svc.ListVisible(ctx, services.ListConnectionsOptions{
		UserID:         userID,
		ProtocolID:     c.Query("protocol_id"),
		TeamID:         strings.TrimSpace(c.Query("team_id")),
		FolderID:       c.Query("folder_id"),
		Search:         c.Query("search"),
		IncludeTargets: includeTargets,
		IncludeGrants:  includeGrants,
		Page:           page,
		PerPage:        perPage,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	meta := &response.Meta{
		Page:       result.Page,
		PerPage:    result.PerPage,
		Total:      int(result.Total),
		TotalPages: computeTotalPages(result.Total, int64(result.PerPage)),
	}
	response.SuccessWithMeta(c, http.StatusOK, result.Connections, meta)
}

// Create registers a new connection for the authenticated user.
func (h *ConnectionHandler) Create(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	var payload createConnectionPayload
	if !bindAndValidate(c, &payload) {
		return
	}

	ctx := requestContext(c)

	connection, err := h.svc.Create(ctx, userID, services.CreateConnectionInput{
		Name:        payload.Name,
		Description: payload.Description,
		ProtocolID:  payload.ProtocolID,
		TeamID:      payload.TeamID,
		FolderID:    payload.FolderID,
		Metadata:    payload.Metadata,
		Settings:    payload.Settings,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	if h.shareSvc != nil && connection.TeamID != nil && len(payload.GrantTeamPermissions) > 0 {
		pruned := dedupePermissions(payload.GrantTeamPermissions)
		if len(pruned) > 0 {
			if _, err := h.shareSvc.CreateShare(ctx, userID, connection.ID, services.CreateShareInput{
				PrincipalType: services.PrincipalTypeTeam,
				PrincipalID:   *connection.TeamID,
				PermissionIDs: pruned,
			}); err != nil {
				response.Error(c, err)
				return
			}
		}
	}

	response.Success(c, http.StatusCreated, connection)
}

// Summary returns connection counts grouped by protocol for the authenticated user.
func (h *ConnectionHandler) Summary(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	ctx := requestContext(c)
	counts, err := h.svc.CountByProtocol(ctx, services.ListConnectionsOptions{
		UserID: userID,
		TeamID: strings.TrimSpace(c.Query("team_id")),
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	summaries := make([]protocolCount, 0, len(counts))
	for protocolID, count := range counts {
		summaries = append(summaries, protocolCount{
			ProtocolID: protocolID,
			Count:      count,
		})
	}

	response.Success(c, http.StatusOK, summaries)
}

// Get fetches a single connection if the user can access it.
func (h *ConnectionHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	includeTargets, includeGrants := parseIncludes(c.Query("include"))

	ctx := requestContext(c)
	connection, err := h.svc.GetVisible(ctx, userID, c.Param("id"), includeTargets, includeGrants)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, connection)
}

func parseIncludes(includeParam string) (bool, bool) {
	if includeParam == "" {
		return true, false
	}

	includeTargets := false
	includeGrants := false
	for _, part := range strings.Split(includeParam, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "targets":
			includeTargets = true
		case "shares":
			includeGrants = true
		}
	}
	return includeTargets, includeGrants
}

func computeTotalPages(total, perPage int64) int {
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

type protocolCount struct {
	ProtocolID string `json:"protocol_id"`
	Count      int64  `json:"count"`
}

type createConnectionPayload struct {
	Name                 string         `json:"name" binding:"required"`
	Description          string         `json:"description"`
	ProtocolID           string         `json:"protocol_id" binding:"required"`
	TeamID               *string        `json:"team_id"`
	FolderID             *string        `json:"folder_id"`
	Metadata             map[string]any `json:"metadata"`
	Settings             map[string]any `json:"settings"`
	GrantTeamPermissions []string       `json:"grant_team_permissions"`
}

func dedupePermissions(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}
