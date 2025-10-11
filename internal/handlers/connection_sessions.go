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
		permissionManage = "permission.manage"
		connectionManage = "connection.manage"
	)

	isAdmin := false
	if h.checker != nil {
		if hasAdmin, err := h.checker.Check(ctx, userID, permissionManage); err != nil {
			response.Error(c, err)
			return
		} else if hasAdmin {
			isAdmin = true
		} else if hasConnManage, err := h.checker.Check(ctx, userID, connectionManage); err != nil {
			response.Error(c, err)
			return
		} else if hasConnManage {
			isAdmin = true
		}
	}

	sessions := h.sessions.ListActive(services.ListActiveOptions{
		UserID:     userID,
		IncludeAll: isAdmin,
	})

	protocolFilter := strings.TrimSpace(c.Query("protocol_id"))
	teamFilter := strings.TrimSpace(c.Query("team_id"))

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
