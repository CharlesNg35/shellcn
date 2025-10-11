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
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	ProtocolID  string                    `json:"protocol_id"`
	TeamID      *string                   `json:"team_id"`
	OwnerUserID string                    `json:"owner_user_id"`
	FolderID    *string                   `json:"folder_id"`
	Metadata    map[string]any            `json:"metadata,omitempty"`
	Settings    map[string]any            `json:"settings,omitempty"`
	SecretID    *string                   `json:"secret_id"`
	LastUsedAt  *time.Time                `json:"last_used_at,omitempty"`
	Targets     []ConnectionTargetDTO     `json:"targets,omitempty"`
	Visibility  []ConnectionVisibilityDTO `json:"visibility,omitempty"`
	Folder      *ConnectionFolderDTO      `json:"folder,omitempty"`
}

// ConnectionTargetDTO returns target metadata for API consumers.
type ConnectionTargetDTO struct {
	ID     string            `json:"id"`
	Host   string            `json:"host"`
	Port   int               `json:"port"`
	Labels map[string]string `json:"labels,omitempty"`
	Order  int               `json:"ordering"`
}

// ConnectionVisibilityDTO models ACL style visibility records.
type ConnectionVisibilityDTO struct {
	ID              string  `json:"id"`
	TeamID          *string `json:"team_id"`
	UserID          *string `json:"user_id"`
	PermissionScope string  `json:"permission_scope"`
}

// ConnectionFolderDTO summarizes folder metadata.
type ConnectionFolderDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description"`
	ParentID    *string        `json:"parent_id"`
	TeamID      *string        `json:"team_id"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ListConnectionsOptions defines filters for connection lookups.
type ListConnectionsOptions struct {
	UserID            string
	ProtocolID        string
	TeamID            string
	FolderID          string
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

// CreateConnectionInput describes the fields needed to create a connection.
type CreateConnectionInput struct {
	Name        string
	Description string
	ProtocolID  string
	TeamID      *string
	FolderID    *string
	Metadata    map[string]any
	Settings    map[string]any
}

// NewConnectionService constructs a ConnectionService.
func NewConnectionService(db *gorm.DB, checker PermissionChecker) (*ConnectionService, error) {
	if db == nil {
		return nil, errors.New("connection service: db is required")
	}
	return &ConnectionService{db: db, checker: checker}, nil
}

// Create registers a new connection owned by the supplied user.
func (s *ConnectionService) Create(ctx context.Context, userID string, input CreateConnectionInput) (*ConnectionDTO, error) {
	ctx = ensureContext(ctx)
	canManage, err := s.canManageConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, apperrors.ErrForbidden
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, apperrors.NewBadRequest("connection name is required")
	}

	protocolID := strings.TrimSpace(input.ProtocolID)
	if protocolID == "" {
		return nil, apperrors.NewBadRequest("protocol id is required")
	}

	connection := models.Connection{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		ProtocolID:  protocolID,
		OwnerUserID: userID,
	}

	if teamID := normalizeOptionalID(input.TeamID); teamID != nil {
		connection.TeamID = teamID
	}
	if folderID := normalizeOptionalID(input.FolderID); folderID != nil {
		connection.FolderID = folderID
	}

	if input.Metadata != nil {
		data, marshalErr := json.Marshal(input.Metadata)
		if marshalErr != nil {
			return nil, apperrors.NewBadRequest("invalid metadata payload")
		}
		connection.Metadata = datatypes.JSON(data)
	}

	if input.Settings != nil {
		data, marshalErr := json.Marshal(input.Settings)
		if marshalErr != nil {
			return nil, apperrors.NewBadRequest("invalid settings payload")
		}
		connection.Settings = datatypes.JSON(data)
	}

	if err := s.db.WithContext(ctx).Create(&connection).Error; err != nil {
		return nil, fmt.Errorf("connection service: create connection: %w", err)
	}

	if err := s.db.WithContext(ctx).
		Preload("Folder").
		First(&connection, "id = ?", connection.ID).Error; err != nil {
		return nil, fmt.Errorf("connection service: reload connection: %w", err)
	}

	dto := mapConnection(connection, false, false)
	return &dto, nil
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
		Order("LOWER(connections.name) ASC, connections.created_at DESC, connections.id ASC").
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

// CountByFolder returns visible connection counts keyed by folder ID (nil => "unassigned").
func (s *ConnectionService) CountByFolder(ctx context.Context, opts ListConnectionsOptions) (map[string]int64, error) {
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
		return map[string]int64{}, nil
	}

	manageAll, err := s.canManageConnections(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}

	query := s.applyFilters(
		s.db.WithContext(ctx).Model(&models.Connection{}),
		opts,
		userCtx,
		manageAll,
	)

	type row struct {
		FolderID *string
		Count    int64
	}

	var rows []row
	if err := query.Select("folder_id, COUNT(*) as count").Group("folder_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("connection service: count by folder: %w", err)
	}

	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		key := "unassigned"
		if r.FolderID != nil && *r.FolderID != "" {
			key = *r.FolderID
		}
		result[key] = r.Count
	}
	return result, nil
}

// CountByProtocol returns visible connection counts keyed by protocol ID.
func (s *ConnectionService) CountByProtocol(ctx context.Context, opts ListConnectionsOptions) (map[string]int64, error) {
	ctx = ensureContext(ctx)
	userCtx, err := s.userContext(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}

	canView, err := s.canViewConnections(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}
	if !canView && !userCtx.IsRoot {
		return map[string]int64{}, nil
	}

	manageAll, err := s.canManageConnections(ctx, opts.UserID)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).
		Model(&models.Connection{}).
		Select("connections.protocol_id, COUNT(DISTINCT connections.id) AS total").
		Group("connections.protocol_id")

	query = s.applyFilters(query, opts, userCtx, manageAll)

	var rows []struct {
		ProtocolID string
		Total      int64
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("connection service: count by protocol: %w", err)
	}

	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row.ProtocolID == "" {
			continue
		}
		result[row.ProtocolID] = row.Total
	}

	return result, nil
}

func (s *ConnectionService) preloadScopes(ctx context.Context, opts ListConnectionsOptions) *gorm.DB {
	db := s.db.WithContext(ctx).Model(&models.Connection{})
	db = db.Preload("Folder")
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

	if teamID := strings.TrimSpace(opts.TeamID); teamID != "" {
		if strings.EqualFold(teamID, "personal") {
			db = db.Where("connections.team_id IS NULL")
		} else {
			db = db.Where("connections.team_id = ?", teamID)
		}
	}

	if folderID := strings.TrimSpace(opts.FolderID); folderID != "" {
		if folderID == "unassigned" {
			db = db.Where("connections.folder_id IS NULL")
		} else {
			db = db.Where("connections.folder_id = ?", folderID)
		}
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

	if len(userCtx.TeamIDs) > 0 {
		connClauses = append(connClauses, "connections.team_id IN ?")
		connArgs = append(connArgs, userCtx.TeamIDs)
	}

	db = db.Joins("LEFT JOIN connection_visibilities vis ON vis.connection_id = connections.id")

	visClauses := []string{"vis.user_id = ?"}
	visArgs := []any{userCtx.ID}
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
		ID:      user.ID,
		IsRoot:  user.IsRoot,
		TeamIDs: teamIDs,
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

func normalizeOptionalID(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	result := trimmed
	return &result
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
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		ProtocolID:  row.ProtocolID,
		TeamID:      row.TeamID,
		OwnerUserID: row.OwnerUserID,
		FolderID:    row.FolderID,
		Metadata:    decodeJSONMap(row.Metadata),
		Settings:    decodeJSONMap(row.Settings),
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
	if row.Folder != nil {
		dto.Folder = &ConnectionFolderDTO{
			ID:          row.Folder.ID,
			Name:        row.Folder.Name,
			Slug:        row.Folder.Slug,
			Description: row.Folder.Description,
			ParentID:    row.Folder.ParentID,
			TeamID:      row.Folder.TeamID,
			Metadata:    decodeJSONMap(row.Folder.Metadata),
		}
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
		dto.Labels = decodeJSONMapString(target.Labels)
		targets = append(targets, dto)
	}
	return targets
}

func mapVisibility(rows []models.ConnectionVisibility) []ConnectionVisibilityDTO {
	items := make([]ConnectionVisibilityDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, ConnectionVisibilityDTO{
			ID:              row.ID,
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

func decodeJSONMapString(value datatypes.JSON) map[string]string {
	if len(value) == 0 {
		return nil
	}
	result := make(map[string]string)
	var raw map[string]any
	if err := json.Unmarshal(value, &raw); err != nil {
		return nil
	}
	for key, val := range raw {
		switch typed := val.(type) {
		case string:
			result[key] = typed
		default:
			b, err := json.Marshal(typed)
			if err == nil {
				result[key] = string(b)
			}
		}
	}
	return result
}

type userContext struct {
	ID      string
	IsRoot  bool
	TeamIDs []string
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
