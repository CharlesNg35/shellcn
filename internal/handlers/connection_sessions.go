package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ActiveConnectionHandler exposes active connection session endpoints.
type ActiveConnectionHandler struct {
	sessions *services.ActiveSessionService
	checker  *permissions.Checker
}

// NewActiveConnectionHandler constructs a handler for active connection sessions.
func NewActiveConnectionHandler(svc *services.ActiveSessionService, checker *permissions.Checker) *ActiveConnectionHandler {
	return &ActiveConnectionHandler{
		sessions: svc,
		checker:  checker,
	}
}

// ListActive returns active connection sessions visible to the authenticated user.
func (h *ActiveConnectionHandler) ListActive(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	if h.sessions == nil {
		response.Success(c, http.StatusOK, []services.ActiveSessionRecord{})
		return
	}

	ctx := requestContext(c)

	const (
		permissionManage      = "permission.manage"
		connectionViewAll     = "connection.view_all"
		connectionManage      = "connection.manage"
		sessionActiveViewAll  = "session.active.view_all"
		sessionActiveViewTeam = "session.active.view_team"
		scopeDefault          = "default"
		scopePersonal         = "personal"
		scopeTeam             = "team"
		scopeAll              = "all"
	)

	scopeValue := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	scope := scopeDefault
	switch scopeValue {
	case scopePersonal, "self", "me":
		scope = scopePersonal
	case scopeTeam, "teams":
		scope = scopeTeam
	case scopeAll, "global":
		scope = scopeAll
	default:
		scope = scopeDefault
	}

	protocolFilter := strings.TrimSpace(c.Query("protocol_id"))
	teamFilter := strings.TrimSpace(c.Query("team_id"))

	checkPermission := func(permissionID string) (bool, error) {
		if h.checker == nil || strings.TrimSpace(permissionID) == "" {
			return false, nil
		}
		return h.checker.Check(ctx, userID, permissionID)
	}

	adminPerms := []string{permissionManage, connectionViewAll, connectionManage}

	includeAll := false
	includeTeams := false
	var teamIDs []string

	switch scope {
	case scopeAll:
		allowed := false
		if ok, err := checkPermission(sessionActiveViewAll); err != nil {
			response.Error(c, err)
			return
		} else if ok {
			allowed = true
		}
		if !allowed {
			for _, permID := range adminPerms {
				ok, err := checkPermission(permID)
				if err != nil {
					response.Error(c, err)
					return
				}
				if ok {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			response.Error(c, errors.ErrForbidden)
			return
		}
		includeAll = true
	case scopeTeam:
		hasAll, err := checkPermission(sessionActiveViewAll)
		if err != nil {
			response.Error(c, err)
			return
		}
		if hasAll {
			includeAll = true
			break
		}

		allowed := false
		if ok, err := checkPermission(sessionActiveViewTeam); err != nil {
			response.Error(c, err)
			return
		} else if ok {
			allowed = true
		}
		if !allowed {
			for _, permID := range adminPerms {
				ok, err := checkPermission(permID)
				if err != nil {
					response.Error(c, err)
					return
				}
				if ok {
					allowed = true
					includeAll = true
					break
				}
			}
		}
		if !allowed {
			response.Error(c, errors.ErrForbidden)
			return
		}
		if !includeAll {
			if h.checker == nil {
				response.Error(c, errors.ErrForbidden)
				return
			}
			ids, err := h.checker.GetUserTeamIDs(ctx, userID)
			if err != nil {
				response.Error(c, err)
				return
			}
			if len(ids) == 0 {
				response.Success(c, http.StatusOK, []services.ActiveSessionRecord{})
				return
			}
			if teamFilter != "" && !strings.EqualFold(teamFilter, "personal") {
				selected := ""
				for _, id := range ids {
					if strings.EqualFold(strings.TrimSpace(id), teamFilter) {
						selected = id
						break
					}
				}
				if selected == "" {
					response.Error(c, errors.ErrForbidden)
					return
				}
				ids = []string{selected}
			}
			includeTeams = true
			teamIDs = ids
		}
	case scopePersonal:
		// Explicit personal scope limits to caller's own sessions only.
	default:
		// Legacy behaviour - admins automatically see all sessions.
		hasAll, err := checkPermission(sessionActiveViewAll)
		if err != nil {
			response.Error(c, err)
			return
		}
		if hasAll {
			includeAll = true
		} else {
			for _, permID := range adminPerms {
				ok, err := checkPermission(permID)
				if err != nil {
					response.Error(c, err)
					return
				}
				if ok {
					includeAll = true
					break
				}
			}
		}
	}

	sessions := h.sessions.ListActive(services.ListActiveOptions{
		UserID:       userID,
		IncludeAll:   includeAll,
		IncludeTeams: includeTeams,
		TeamIDs:      teamIDs,
	})

	filtered := make([]services.ActiveSessionRecord, 0, len(sessions))
	for _, session := range sessions {
		if protocolFilter != "" && !strings.EqualFold(session.ProtocolID, protocolFilter) {
			continue
		}
		if teamFilter != "" {
			if strings.EqualFold(teamFilter, "personal") {
				if session.TeamID != nil && strings.TrimSpace(*session.TeamID) != "" {
					continue
				}
			} else {
				if session.TeamID == nil || !strings.EqualFold(strings.TrimSpace(*session.TeamID), teamFilter) {
					continue
				}
			}
		}
		filtered = append(filtered, session)
	}

	response.Success(c, http.StatusOK, filtered)
}
