package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ConnectionHandler exposes connection APIs.
type ConnectionHandler struct {
	svc *services.ConnectionService
}

// NewConnectionHandler constructs a handler using the provided dependencies.
func NewConnectionHandler(db *gorm.DB, checker services.PermissionChecker) (*ConnectionHandler, error) {
	svc, err := services.NewConnectionService(db, checker)
	if err != nil {
		return nil, err
	}
	return &ConnectionHandler{svc: svc}, nil
}

// List returns visible connections for the authenticated user.
func (h *ConnectionHandler) List(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	includeTargets, includeVisibility := parseIncludes(c.Query("include"))
	page := parseIntQuery(c, "page", 1)
	perPage := parseIntQuery(c, "per_page", 25)

	result, err := h.svc.ListVisible(c.Request.Context(), services.ListConnectionsOptions{
		UserID:            userID,
		ProtocolID:        c.Query("protocol_id"),
		Search:            c.Query("search"),
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
		TotalPages: computeTotalPages(result.Total, int64(result.PerPage)),
	}
	response.SuccessWithMeta(c, http.StatusOK, result.Connections, meta)
}

// Get fetches a single connection if the user can access it.
func (h *ConnectionHandler) Get(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	includeTargets, includeVisibility := parseIncludes(c.Query("include"))

	connection, err := h.svc.GetVisible(c.Request.Context(), userID, c.Param("id"), includeTargets, includeVisibility)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, http.StatusOK, connection)
}

func parseIncludes(includeParam string) (bool, bool) {
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

func parseIntQuery(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return fallback
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
