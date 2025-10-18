package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type SnippetHandler struct {
	svc       *services.SnippetService
	checker   *permissions.Checker
	lifecycle *services.SessionLifecycleService
	active    *services.ActiveSessionService
}

func NewSnippetHandler(
	svc *services.SnippetService,
	checker *permissions.Checker,
	lifecycle *services.SessionLifecycleService,
	active *services.ActiveSessionService,
) *SnippetHandler {
	return &SnippetHandler{
		svc:       svc,
		checker:   checker,
		lifecycle: lifecycle,
		active:    active,
	}
}

type snippetDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Command      string  `json:"command"`
	Scope        string  `json:"scope"`
	OwnerUserID  *string `json:"owner_user_id,omitempty"`
	ConnectionID *string `json:"connection_id,omitempty"`
	UpdatedAt    string  `json:"updated_at"`
}

type stdinHandle interface {
	drivers.SessionHandle
	Stdin() io.WriteCloser
}

func mapSnippet(snippet *models.Snippet) snippetDTO {
	updatedAt := ""
	if !snippet.UpdatedAt.IsZero() {
		updatedAt = snippet.UpdatedAt.Format(time.RFC3339)
	}
	return snippetDTO{
		ID:           snippet.ID,
		Name:         snippet.Name,
		Description:  snippet.Description,
		Command:      snippet.Command,
		Scope:        snippet.Scope,
		OwnerUserID:  snippet.OwnerUserID,
		ConnectionID: snippet.ConnectionID,
		UpdatedAt:    updatedAt,
	}
}

func normalizeScopeValue(scope string) string {
	return strings.ToLower(strings.TrimSpace(scope))
}

func (h *SnippetHandler) manageSnippetsAllowed(ctx context.Context, userID string) (bool, error) {
	if h == nil || h.checker == nil {
		return false, nil
	}
	return h.checker.Check(ctx, userID, "protocol:ssh.manage_snippets")
}

func (h *SnippetHandler) connectionSnippetAllowed(ctx context.Context, userID, connectionID string) (bool, error) {
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" {
		return false, nil
	}
	allowed, err := h.manageSnippetsAllowed(ctx, userID)
	if err != nil {
		return false, err
	}
	if !allowed || h.checker == nil {
		return false, nil
	}
	return h.checker.CheckResource(ctx, userID, "connection", connectionID, "connection.manage")
}

func (h *SnippetHandler) ensureSnippetEditPermission(ctx context.Context, userID string, snippet *models.Snippet) error {
	if snippet == nil {
		return apperrors.ErrForbidden
	}
	scope := normalizeScopeValue(snippet.Scope)
	switch scope {
	case "global":
		allowed, err := h.manageSnippetsAllowed(ctx, userID)
		if err != nil {
			return err
		}
		if !allowed {
			return apperrors.ErrForbidden
		}
	case "connection":
		if snippet.ConnectionID == nil {
			return apperrors.NewBadRequest("snippet is missing connection id")
		}
		allowed, err := h.connectionSnippetAllowed(ctx, userID, *snippet.ConnectionID)
		if err != nil {
			return err
		}
		if !allowed {
			return apperrors.ErrForbidden
		}
	case "user":
		if snippet.OwnerUserID == nil || !strings.EqualFold(*snippet.OwnerUserID, userID) {
			return apperrors.ErrForbidden
		}
	default:
		return apperrors.ErrForbidden
	}
	return nil
}

func (h *SnippetHandler) ensureScopePermissions(ctx context.Context, userID, scope, connectionID string) (string, error) {
	normalized := normalizeScopeValue(scope)
	if normalized == "" {
		normalized = "user"
	}

	switch normalized {
	case "user":
		return normalized, nil
	case "global":
		allowed, err := h.manageSnippetsAllowed(ctx, userID)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "", apperrors.ErrForbidden
		}
		return normalized, nil
	case "connection":
		if strings.TrimSpace(connectionID) == "" {
			return "", apperrors.NewBadRequest("connection_id is required for connection scope")
		}
		allowed, err := h.connectionSnippetAllowed(ctx, userID, connectionID)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "", apperrors.ErrForbidden
		}
		return normalized, nil
	default:
		return "", apperrors.NewBadRequest("invalid scope")
	}
}

func (h *SnippetHandler) userCanWriteSession(record *services.ActiveSessionRecord, userID string) bool {
	if record == nil {
		return false
	}
	if strings.EqualFold(record.OwnerUserID, userID) || strings.EqualFold(record.UserID, userID) {
		return true
	}
	if strings.EqualFold(record.WriteHolder, userID) {
		return true
	}
	if record.Participants != nil {
		if participant, ok := record.Participants[userID]; ok && participant != nil {
			if strings.EqualFold(participant.AccessMode, "write") {
				return true
			}
		}
	}
	return false
}

// List handles GET /api/snippets
func (h *SnippetHandler) List(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	scope := normalizeScopeValue(c.DefaultQuery("scope", "all"))
	connectionID := strings.TrimSpace(c.Query("connection_id"))
	ctx := requestContext(c)

	manageSnippets, err := h.manageSnippetsAllowed(ctx, userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	includeGlobal := false
	includeConnection := false
	includeUser := false

	switch scope {
	case "", "all":
		includeUser = true
		if manageSnippets {
			includeGlobal = true
		}
		if connectionID != "" {
			allowed, err := h.connectionSnippetAllowed(ctx, userID, connectionID)
			if err != nil {
				response.Error(c, err)
				return
			}
			includeConnection = allowed
		}
	case "global":
		if !manageSnippets {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		includeGlobal = true
	case "connection":
		if connectionID == "" {
			response.Error(c, apperrors.NewBadRequest("connection_id is required for connection scope"))
			return
		}
		allowed, err := h.connectionSnippetAllowed(ctx, userID, connectionID)
		if err != nil {
			response.Error(c, err)
			return
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
		includeConnection = true
	case "user":
		includeUser = true
	default:
		response.Error(c, apperrors.NewBadRequest("invalid scope"))
		return
	}

	snippets, err := h.svc.List(ctx, services.ListSnippetsOptions{
		Scope:             scope,
		ConnectionID:      connectionID,
		OwnerUserID:       userID,
		IncludeGlobal:     includeGlobal,
		IncludeConnection: includeConnection,
		IncludeUser:       includeUser,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	dtos := make([]snippetDTO, 0, len(snippets))
	for i := range snippets {
		snippet := snippets[i]
		dtos = append(dtos, mapSnippet(&snippet))
	}

	response.Success(c, http.StatusOK, dtos)
}

type createSnippetRequest struct {
	Name         string  `json:"name" binding:"required"`
	Description  string  `json:"description"`
	Command      string  `json:"command" binding:"required"`
	Scope        string  `json:"scope"`
	ConnectionID *string `json:"connection_id"`
}

// Create handles POST /api/snippets
func (h *SnippetHandler) Create(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	var body createSnippetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid snippet payload"))
		return
	}

	ctx := requestContext(c)

	connectionID := ""
	if body.ConnectionID != nil {
		connectionID = strings.TrimSpace(*body.ConnectionID)
	}

	scope, err := h.ensureScopePermissions(ctx, userID, body.Scope, connectionID)
	if err != nil {
		response.Error(c, err)
		return
	}

	input := services.CreateSnippetInput{
		Name:         body.Name,
		Description:  body.Description,
		Command:      body.Command,
		Scope:        scope,
		OwnerUserID:  userID,
		ConnectionID: connectionID,
	}

	snippet, err := h.svc.Create(ctx, input)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusCreated, mapSnippet(snippet))
}

type updateSnippetRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	Command      *string `json:"command"`
	Scope        *string `json:"scope"`
	ConnectionID *string `json:"connection_id"`
}

// Update handles PUT /api/snippets/:id
func (h *SnippetHandler) Update(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	snippetID := strings.TrimSpace(c.Param("id"))
	if snippetID == "" {
		response.Error(c, apperrors.NewBadRequest("snippet id is required"))
		return
	}

	var body updateSnippetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid snippet payload"))
		return
	}

	ctx := requestContext(c)

	snippet, err := h.svc.Get(ctx, snippetID)
	if err != nil {
		if errors.Is(err, services.ErrSnippetNotFound) {
			response.Error(c, apperrors.ErrNotFound)
			return
		}
		response.Error(c, err)
		return
	}

	if err := h.ensureSnippetEditPermission(ctx, userID, snippet); err != nil {
		response.Error(c, err)
		return
	}

	desiredScope := snippet.Scope
	desiredConnectionID := ""
	if snippet.ConnectionID != nil {
		desiredConnectionID = strings.TrimSpace(*snippet.ConnectionID)
	}

	if body.ConnectionID != nil {
		desiredConnectionID = strings.TrimSpace(*body.ConnectionID)
	}

	if body.Scope != nil {
		normalizedScope, err := h.ensureScopePermissions(ctx, userID, *body.Scope, desiredConnectionID)
		if err != nil {
			response.Error(c, err)
			return
		}
		desiredScope = normalizedScope
	} else if normalizeScopeValue(desiredScope) == "connection" && body.ConnectionID != nil {
		// ensure permissions for new connection
		if _, err := h.ensureScopePermissions(ctx, userID, desiredScope, desiredConnectionID); err != nil {
			response.Error(c, err)
			return
		}
	}

	input := services.UpdateSnippetInput{
		Name:         body.Name,
		Description:  body.Description,
		Command:      body.Command,
		Scope:        body.Scope,
		ConnectionID: body.ConnectionID,
		OwnerUserID:  userID,
	}

	if body.Scope != nil {
		input.Scope = &desiredScope
		if normalizeScopeValue(desiredScope) != "connection" {
			input.ConnectionID = nil
		} else if body.ConnectionID == nil && snippet.ConnectionID != nil {
			existing := strings.TrimSpace(*snippet.ConnectionID)
			input.ConnectionID = &existing
		}
	} else if normalizeScopeValue(desiredScope) == "connection" && body.ConnectionID == nil && snippet.ConnectionID != nil {
		existing := strings.TrimSpace(*snippet.ConnectionID)
		input.ConnectionID = &existing
	}

	updated, err := h.svc.Update(ctx, snippetID, input)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, mapSnippet(updated))
}

// Delete handles DELETE /api/snippets/:id
func (h *SnippetHandler) Delete(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	snippetID := strings.TrimSpace(c.Param("id"))
	if snippetID == "" {
		response.Error(c, apperrors.NewBadRequest("snippet id is required"))
		return
	}

	ctx := requestContext(c)

	snippet, err := h.svc.Get(ctx, snippetID)
	if err != nil {
		if errors.Is(err, services.ErrSnippetNotFound) {
			response.Error(c, apperrors.ErrNotFound)
			return
		}
		response.Error(c, err)
		return
	}

	if err := h.ensureSnippetEditPermission(ctx, userID, snippet); err != nil {
		response.Error(c, err)
		return
	}

	if err := h.svc.Delete(ctx, snippetID); err != nil {
		response.Error(c, err)
		return
	}

	response.Success(c, http.StatusOK, gin.H{"deleted": true})
}

type executeSnippetRequest struct {
	SnippetID string `json:"snippet_id" binding:"required"`
}

// Execute handles POST /api/active-sessions/:id/snippet
func (h *SnippetHandler) Execute(c *gin.Context) {
	if h == nil || h.svc == nil || h.lifecycle == nil || h.active == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	var body executeSnippetRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid snippet payload"))
		return
	}

	snippetID := strings.TrimSpace(body.SnippetID)
	if snippetID == "" {
		response.Error(c, apperrors.NewBadRequest("snippet_id is required"))
		return
	}

	ctx := requestContext(c)

	session, err := h.lifecycle.AuthorizeSessionAccess(ctx, sessionID, userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	activeRecord, ok := h.active.GetSession(sessionID)
	if !ok {
		response.Error(c, apperrors.New("session.tunnel_unavailable", "Session tunnel is not ready", http.StatusConflict))
		return
	}

	if !h.userCanWriteSession(activeRecord, userID) {
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	snippet, err := h.svc.Get(ctx, snippetID)
	if err != nil {
		if errors.Is(err, services.ErrSnippetNotFound) {
			response.Error(c, apperrors.ErrNotFound)
			return
		}
		response.Error(c, err)
		return
	}

	switch normalizeScopeValue(snippet.Scope) {
	case "global":
		allowed, err := h.manageSnippetsAllowed(ctx, userID)
		if err != nil {
			response.Error(c, err)
			return
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	case "connection":
		if snippet.ConnectionID == nil || !strings.EqualFold(strings.TrimSpace(*snippet.ConnectionID), strings.TrimSpace(session.ConnectionID)) {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	case "user":
		if snippet.OwnerUserID == nil || !strings.EqualFold(strings.TrimSpace(*snippet.OwnerUserID), userID) {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	default:
		response.Error(c, apperrors.ErrForbidden)
		return
	}

	handle, ok := h.active.PeekHandle(sessionID)
	if !ok {
		response.Error(c, apperrors.New("session.handle_unavailable", "Session input stream is not ready", http.StatusConflict))
		return
	}

	writer, ok := handle.(stdinHandle)
	if !ok {
		response.Error(c, apperrors.New("session.handle_incompatible", "Session does not support command injection", http.StatusConflict))
		return
	}

	stdin := writer.Stdin()
	if stdin == nil {
		response.Error(c, apperrors.New("session.stdin_missing", "Session input stream unavailable", http.StatusConflict))
		return
	}

	command := snippet.Command
	if !strings.HasSuffix(command, "\n") {
		command += "\n"
	}
	if _, err := io.WriteString(stdin, command); err != nil {
		response.Error(c, apperrors.Wrap(err, "write snippet command"))
		return
	}

	response.Success(c, http.StatusAccepted, gin.H{"executed": true})
}
