package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

// ConnectionDTO represents a sanitized connection payload for API responses.
type ConnectionDTO struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description"`
	ProtocolID     string                    `json:"protocol_id"`
	OrganizationID *string                   `json:"organization_id"`
	TeamID         *string                   `json:"team_id"`
	OwnerUserID    string                    `json:"owner_user_id"`
	Metadata       map[string]any            `json:"metadata,omitempty"`
	Settings       map[string]any            `json:"settings,omitempty"`
	SecretID       *string                   `json:"secret_id"`
	LastUsedAt     *time.Time                `json:"last_used_at,omitempty"`
	Targets        []ConnectionTargetDTO     `json:"targets,omitempty"`
	Visibility     []ConnectionVisibilityDTO `json:"visibility,omitempty"`
}

// ConnectionTargetDTO returns target metadata for API consumers.
type ConnectionTargetDTO struct {
	ID     string         `json:"id"`
	Host   string         `json:"host"`
	Port   int            `json:"port"`
	Labels map[string]any `json:"labels,omitempty"`
	Order  int            `json:"ordering"`
}

// ConnectionVisibilityDTO models ACL style visibility records.
type ConnectionVisibilityDTO struct {
	ID              string  `json:"id"`
	OrganizationID  *string `json:"organization_id"`
	TeamID          *string `json:"team_id"`
	UserID          *string `json:"user_id"`
	PermissionScope string  `json:"permission_scope"`
}

// ListConnectionsOptions defines filters for connection lookups.
type ListConnectionsOptions struct {
	UserID            string
	ProtocolID        string
	Search            string
	IncludeTargets    bool
	IncludeVisibility bool
	Page              int
	PerPage           int
}

// ListConnectionsResult describes a paginated result set.
type ListConnectionsResult struct {
	Connections []ConnectionDTO
	Total       int64
	Page        int
	PerPage     int
}

// ConnectionService orchestrates read operations for connections.
type ConnectionService struct {
	db      *gorm.DB
	checker PermissionChecker
}

// NewConnectionService constructs a ConnectionService.
func NewConnectionService(db *gorm.DB, checker PermissionChecker) (*ConnectionService, error) {
	if db == nil {
		return nil, errors.New("connection service: db is required")
	}
	return &ConnectionService{db: db, checker: checker}, nil
}

// ListVisible returns connections accessible to the supplied user, applying optional filters.
func (s *ConnectionService) ListVisible(ctx context.Context, opts ListConnectionsOptions) (*ListConnectionsResult, error) {
	ctx = ensureContext(ctx)
	userCtx, err := s.userContext(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}

	canView, err := s.canViewConnections(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return &ListConnectionsResult{
			Connections: []ConnectionDTO{},
			Total:       0,
			Page:        sanitizePage(opts.Page),
			PerPage:     sanitizePerPage(opts.PerPage),
		}, nil
	}

	manageAll, err := s.canManageConnections(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}

	base := s.db.WithContext(ctx).Model(&models.Connection{}).Distinct("connections.id")
	filtered := s.applyFilters(base, opts, userCtx, manageAll)

	var total int64
	if err := filtered.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("connection service: count connections: %w", err)
	}

	perPage := sanitizePerPage(opts.PerPage)
	page := sanitizePage(opts.Page)
	offset := (page - 1) * perPage

	dataQuery := s.applyFilters(s.preloadScopes(ctx, opts), opts, userCtx, manageAll).
		Order("connections.created_at DESC").
		Limit(perPage).
		Offset(offset)

	var rows []models.Connection
	if err := dataQuery.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("connection service: list connections: %w", err)
	}

	return &ListConnectionsResult{
		Connections: mapConnections(rows, opts.IncludeTargets, opts.IncludeVisibility),
		Total:       total,
		Page:        page,
		PerPage:     perPage,
	}, nil
}

// GetVisible returns a single connection if the user has access.
func (s *ConnectionService) GetVisible(ctx context.Context, userID, connectionID string, includeTargets, includeVisibility bool) (*ConnectionDTO, error) {
	ctx = ensureContext(ctx)
	userCtx, err := s.userContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	canView, err := s.canViewConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canView && !userCtx.IsRoot {
		return nil, apperrors.ErrForbidden
	}

	manageAll, err := s.canManageConnections(ctx, userID)
	if err != nil {
		return nil, err
	}

	query := s.preloadScopes(ctx, ListConnectionsOptions{
		IncludeTargets:    includeTargets,
		IncludeVisibility: includeVisibility,
	})
	query = query.Where("connections.id = ?", connectionID)
	query = s.applyFilters(query, ListConnectionsOptions{}, userCtx, manageAll)

	var connection models.Connection
	if err := query.First(&connection).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("connection service: load connection: %w", err)
	}

	dto := mapConnection(connection, includeTargets, includeVisibility)
	return &dto, nil
}

func (s *ConnectionService) preloadScopes(ctx context.Context, opts ListConnectionsOptions) *gorm.DB {
	db := s.db.WithContext(ctx).Model(&models.Connection{})
	if opts.IncludeTargets {
		db = db.Preload("Targets")
	}
	if opts.IncludeVisibility {
		db = db.Preload("Visibility")
	}
	return db
}

func (s *ConnectionService) applyFilters(db *gorm.DB, opts ListConnectionsOptions, userCtx userContext, allowAll bool) *gorm.DB {
	if protocol := strings.TrimSpace(opts.ProtocolID); protocol != "" {
		db = db.Where("connections.protocol_id = ?", protocol)
	}

	if search := strings.TrimSpace(opts.Search); search != "" {
		searchLike := "%" + strings.ToLower(search) + "%"
		db = db.Where("(LOWER(connections.name) LIKE ? OR LOWER(connections.description) LIKE ?)", searchLike, searchLike)
	}

	if allowAll || userCtx.IsRoot {
		return db
	}

	connClauses := []string{"connections.owner_user_id = ?"}
	connArgs := []any{userCtx.ID}
	connClauses = append(connClauses, "connections.organization_id IS NULL")

	if userCtx.OrganizationID != nil {
		connClauses = append(connClauses, "connections.organization_id = ?")
		connArgs = append(connArgs, *userCtx.OrganizationID)
	}
	if len(userCtx.TeamIDs) > 0 {
		connClauses = append(connClauses, "connections.team_id IN ?")
		connArgs = append(connArgs, userCtx.TeamIDs)
	}

	db = db.Joins("LEFT JOIN connection_visibilities vis ON vis.connection_id = connections.id")

	visClauses := []string{"vis.user_id = ?"}
	visArgs := []any{userCtx.ID}
	if userCtx.OrganizationID != nil {
		visClauses = append(visClauses, "vis.organization_id = ?")
		visArgs = append(visArgs, *userCtx.OrganizationID)
	}
	if len(userCtx.TeamIDs) > 0 {
		visClauses = append(visClauses, "vis.team_id IN ?")
		visArgs = append(visArgs, userCtx.TeamIDs)
	}

	whereParts := []string{}
	if len(connClauses) > 0 {
		whereParts = append(whereParts, "("+strings.Join(connClauses, " OR ")+")")
	}
	if len(visClauses) > 0 {
		whereParts = append(whereParts, "("+strings.Join(visClauses, " OR ")+")")
	}

	if len(whereParts) > 0 {
		args := append(connArgs, visArgs...)
		db = db.Where(strings.Join(whereParts, " OR "), args...)
	}
	return db
}

func (s *ConnectionService) userContext(ctx context.Context, userID string) (userContext, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return userContext{}, errors.New("connection service: user id is required")
	}

	var user models.User
	if err := s.db.WithContext(ctx).
		Preload("Teams").
		First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return userContext{}, apperrors.ErrNotFound
		}
		return userContext{}, fmt.Errorf("connection service: load user: %w", err)
	}

	teamIDs := make([]string, 0, len(user.Teams))
	for _, team := range user.Teams {
		teamIDs = append(teamIDs, team.ID)
	}

	return userContext{
		ID:             user.ID,
		IsRoot:         user.IsRoot,
		OrganizationID: user.OrganizationID,
		TeamIDs:        teamIDs,
	}, nil
}

func (s *ConnectionService) canViewConnections(ctx context.Context, userID string) (bool, error) {
	if s.checker == nil {
		return true, nil
	}
	return s.checker.Check(ctx, userID, "connection.view")
}

func (s *ConnectionService) canManageConnections(ctx context.Context, userID string) (bool, error) {
	if s.checker == nil {
		return false, nil
	}
	return s.checker.Check(ctx, userID, "connection.manage")
}

func mapConnections(rows []models.Connection, includeTargets, includeVisibility bool) []ConnectionDTO {
	items := make([]ConnectionDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapConnection(row, includeTargets, includeVisibility))
	}
	return items
}

func mapConnection(row models.Connection, includeTargets, includeVisibility bool) ConnectionDTO {
	dto := ConnectionDTO{
		ID:             row.ID,
		Name:           row.Name,
		Description:    row.Description,
		ProtocolID:     row.ProtocolID,
		OrganizationID: row.OrganizationID,
		TeamID:         row.TeamID,
		OwnerUserID:    row.OwnerUserID,
		Metadata:       decodeJSONMap(row.Metadata),
		Settings:       decodeJSONMap(row.Settings),
	}
	if row.SecretID != nil {
		dto.SecretID = row.SecretID
	}
	if row.LastUsedAt != nil {
		timestamp := *row.LastUsedAt
		dto.LastUsedAt = &timestamp
	}

	if includeTargets {
		dto.Targets = mapTargets(row.Targets)
	}
	if includeVisibility {
		dto.Visibility = mapVisibility(row.Visibility)
	}

	return dto
}

func mapTargets(rows []models.ConnectionTarget) []ConnectionTargetDTO {
	targets := make([]ConnectionTargetDTO, 0, len(rows))
	for _, target := range rows {
		dto := ConnectionTargetDTO{
			ID:    target.ID,
			Host:  target.Host,
			Port:  target.Port,
			Order: target.Ordering,
		}
		dto.Labels = decodeJSONMap(target.Labels)
		targets = append(targets, dto)
	}
	return targets
}

func mapVisibility(rows []models.ConnectionVisibility) []ConnectionVisibilityDTO {
	items := make([]ConnectionVisibilityDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, ConnectionVisibilityDTO{
			ID:              row.ID,
			OrganizationID:  row.OrganizationID,
			TeamID:          row.TeamID,
			UserID:          row.UserID,
			PermissionScope: row.PermissionScope,
		})
	}
	return items
}

func decodeJSONMap(value datatypes.JSON) map[string]any {
	if len(value) == 0 {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(value, &result); err != nil {
		return nil
	}
	return result
}

type userContext struct {
	ID             string
	IsRoot         bool
	OrganizationID *string
	TeamIDs        []string
}

func sanitizePerPage(perPage int) int {
	switch {
	case perPage <= 0:
		return 25
	case perPage > 100:
		return 100
	default:
		return perPage
	}
}

func sanitizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}
