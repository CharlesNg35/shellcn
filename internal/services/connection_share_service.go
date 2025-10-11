package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

const (
	PrincipalTypeUser = "user"
	PrincipalTypeTeam = "team"
)

// ShareActor describes a user or team participating in a share.
type ShareActor struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// ConnectionShareDTO represents a connection share grouped per principal.
type ConnectionShareDTO struct {
	ShareID          string         `json:"share_id"`
	Principal        ShareActor     `json:"principal"`
	PermissionScopes []string       `json:"permission_scopes"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	GrantedBy        *ShareActor    `json:"granted_by,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// ConnectionShareService manages resource-scoped permissions for connections.
type ConnectionShareService struct {
	db      *gorm.DB
	checker PermissionChecker
}

// NewConnectionShareService constructs a share service.
func NewConnectionShareService(db *gorm.DB, checker PermissionChecker) (*ConnectionShareService, error) {
	if db == nil {
		return nil, errors.New("connection share service: db is required")
	}
	if checker == nil {
		return nil, errors.New("connection share service: permission checker is required")
	}
	return &ConnectionShareService{db: db, checker: checker}, nil
}

// CreateShareInput describes the payload for creating or replacing a share.
type CreateShareInput struct {
	PrincipalType string
	PrincipalID   string
	PermissionIDs []string
	ExpiresAt     *time.Time
	Metadata      map[string]any
}

// ListShares returns the active resource permissions associated with the connection.
func (s *ConnectionShareService) ListShares(ctx context.Context, requesterID, connectionID string) ([]ConnectionShareDTO, error) {
	ctx = ensureContext(ctx)

	if err := s.ensureShareAccess(ctx, requesterID, connectionID); err != nil {
		return nil, err
	}

	var grants []models.ResourcePermission
	if err := s.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", connectionResourceType, connectionID).
		Find(&grants).Error; err != nil {
		return nil, fmt.Errorf("connection share service: list shares: %w", err)
	}

	return buildShareDTOs(ctx, s.db, grants)
}

// CreateShare replaces resource grants for the specified principal and returns the resulting entry.
func (s *ConnectionShareService) CreateShare(ctx context.Context, requesterID, connectionID string, input CreateShareInput) (*ConnectionShareDTO, error) {
	ctx = ensureContext(ctx)

	if err := s.ensureShareAccess(ctx, requesterID, connectionID); err != nil {
		return nil, err
	}

	principalType := strings.ToLower(strings.TrimSpace(input.PrincipalType))
	principalID := strings.TrimSpace(input.PrincipalID)
	if principalType == "" || principalID == "" {
		return nil, apperrors.NewBadRequest("principal type and id are required")
	}
	if principalType != PrincipalTypeUser && principalType != PrincipalTypeTeam {
		return nil, apperrors.NewBadRequest("principal type must be user or team")
	}

	if err := s.ensurePrincipalExists(ctx, principalType, principalID); err != nil {
		return nil, err
	}

	permissionIDs, err := s.normalisePermissionIDs(input.PermissionIDs)
	if err != nil {
		return nil, err
	}
	if len(permissionIDs) == 0 {
		return nil, apperrors.NewBadRequest("at least one permission scope is required")
	}

	if err := s.ensureGrantorPermissions(ctx, requesterID, connectionID, permissionIDs); err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if input.ExpiresAt != nil {
		exp := input.ExpiresAt.UTC().Truncate(time.Second)
		if exp.Before(time.Now().UTC()) {
			return nil, apperrors.NewBadRequest("expiration must be in the future")
		}
		expiresAt = &exp
	}

	var metadata datatypes.JSON
	if len(input.Metadata) > 0 {
		raw, marshalErr := json.Marshal(input.Metadata)
		if marshalErr != nil {
			return nil, apperrors.NewBadRequest("metadata must be JSON serialisable")
		}
		metadata = datatypes.JSON(raw)
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []models.ResourcePermission
		if err := tx.
			Where("resource_type = ? AND resource_id = ? AND principal_type = ? AND principal_id = ?",
				connectionResourceType, connectionID, principalType, principalID).
			Find(&existing).Error; err != nil {
			return fmt.Errorf("connection share service: load existing grants: %w", err)
		}

		existingByPermission := make(map[string]*models.ResourcePermission, len(existing))
		for i := range existing {
			existingByPermission[existing[i].PermissionID] = &existing[i]
		}

		for permID := range permissionIDs {
			if grant, ok := existingByPermission[permID]; ok {
				updates := map[string]any{
					"expires_at": expiresAt,
					"granted_by_id": func() *string {
						id := requesterID
						return &id
					}(),
				}
				if len(metadata) > 0 {
					updates["metadata"] = metadata
				} else {
					updates["metadata"] = datatypes.JSON(nil)
				}
				if err := tx.Model(grant).Updates(updates).Error; err != nil {
					return fmt.Errorf("connection share service: update grant: %w", err)
				}
				continue
			}

			record := models.ResourcePermission{
				ResourceID:    connectionID,
				ResourceType:  connectionResourceType,
				PrincipalType: principalType,
				PrincipalID:   principalID,
				PermissionID:  permID,
				GrantedByID: func() *string {
					id := requesterID
					return &id
				}(),
				ExpiresAt: expiresAt,
				Metadata:  metadata,
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("connection share service: create grant: %w", err)
			}
		}

		if len(existing) > 0 {
			var toRemoveIDs []string
			for _, grant := range existing {
				if _, keep := permissionIDs[grant.PermissionID]; !keep {
					toRemoveIDs = append(toRemoveIDs, grant.ID)
				}
			}
			if len(toRemoveIDs) > 0 {
				if err := tx.Where("id IN ?", toRemoveIDs).Delete(&models.ResourcePermission{}).Error; err != nil {
					return fmt.Errorf("connection share service: cleanup stale grants: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	var result []models.ResourcePermission
	if err := s.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ? AND principal_type = ? AND principal_id = ?",
			connectionResourceType, connectionID, principalType, principalID).
		Find(&result).Error; err != nil {
		return nil, fmt.Errorf("connection share service: reload grants: %w", err)
	}

	dtos, err := buildShareDTOs(ctx, s.db, result)
	if err != nil {
		return nil, err
	}
	if len(dtos) == 0 {
		return nil, apperrors.ErrNotFound
	}
	return &dtos[0], nil
}

// DeleteShare removes all resource permissions for the aggregated share identifier.
func (s *ConnectionShareService) DeleteShare(ctx context.Context, requesterID, connectionID, shareID string) error {
	ctx = ensureContext(ctx)

	if err := s.ensureShareAccess(ctx, requesterID, connectionID); err != nil {
		return err
	}

	principalType, principalID, err := parseShareID(shareID)
	if err != nil {
		return apperrors.NewBadRequest("invalid share identifier")
	}

	var count int64
	query := s.db.WithContext(ctx).
		Model(&models.ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ? AND principal_type = ? AND principal_id = ?",
			connectionResourceType, connectionID, principalType, principalID)

	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("connection share service: count grants: %w", err)
	}
	if count == 0 {
		return apperrors.ErrNotFound
	}

	if err := query.Delete(&models.ResourcePermission{}).Error; err != nil {
		return fmt.Errorf("connection share service: delete grants: %w", err)
	}
	return nil
}

func (s *ConnectionShareService) ensureShareAccess(ctx context.Context, userID, connectionID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return apperrors.ErrUnauthorized
	}
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" {
		return apperrors.NewBadRequest("connection id is required")
	}

	var connection models.Connection
	if err := s.db.WithContext(ctx).
		Select("id", "owner_user_id", "team_id").
		First(&connection, "id = ?", connectionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrNotFound
		}
		return fmt.Errorf("connection share service: load connection: %w", err)
	}

	if ok, err := s.checker.CheckResource(ctx, userID, connectionResourceType, connectionID, "connection.share"); err != nil {
		return err
	} else if ok {
		return nil
	}

	ok, err := s.checker.Check(ctx, userID, "connection.share")
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.ErrForbidden
	}
	return nil
}

func (s *ConnectionShareService) ensurePrincipalExists(ctx context.Context, principalType, principalID string) error {
	switch principalType {
	case PrincipalTypeUser:
		var user models.User
		if err := s.db.WithContext(ctx).Select("id").First(&user, "id = ?", principalID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.NewBadRequest("user not found")
			}
			return fmt.Errorf("connection share service: load user: %w", err)
		}
	case PrincipalTypeTeam:
		var team models.Team
		if err := s.db.WithContext(ctx).Select("id").First(&team, "id = ?", principalID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.NewBadRequest("team not found")
			}
			return fmt.Errorf("connection share service: load team: %w", err)
		}
	default:
		return apperrors.NewBadRequest("unsupported principal type")
	}
	return nil
}

func (s *ConnectionShareService) normalisePermissionIDs(ids []string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := permissions.Get(trimmed); !ok {
			return nil, apperrors.NewBadRequest(fmt.Sprintf("%s %q", permissions.ErrUnknownPermission.Error(), trimmed))
		}
		result[trimmed] = true

		deps, err := permissions.ResolveDependencies(trimmed)
		if err != nil {
			return nil, apperrors.NewBadRequest(err.Error())
		}
		for _, dep := range deps {
			result[dep] = true
		}
	}
	return result, nil
}

func (s *ConnectionShareService) ensureGrantorPermissions(ctx context.Context, grantorID, connectionID string, permissionIDs map[string]bool) error {
	for id := range permissionIDs {
		ok, err := s.checker.CheckResource(ctx, grantorID, connectionResourceType, connectionID, id)
		if err != nil {
			return err
		}
		if ok {
			continue
		}

		global, err := s.checker.Check(ctx, grantorID, id)
		if err != nil {
			return err
		}
		if !global {
			return apperrors.ErrForbidden
		}
	}
	return nil
}

type shareAggregate struct {
	PrincipalType string
	PrincipalID   string
	Permissions   map[string]struct{}
	ExpiresAt     *time.Time
	Metadata      map[string]any
	GrantedByIDs  map[string]struct{}
}

func buildShareDTOs(ctx context.Context, db *gorm.DB, rows []models.ResourcePermission) ([]ConnectionShareDTO, error) {
	if len(rows) == 0 {
		return []ConnectionShareDTO{}, nil
	}

	aggregates := make(map[string]*shareAggregate)
	userIDs := make(map[string]struct{})
	teamIDs := make(map[string]struct{})
	grantorIDs := make(map[string]struct{})

	for _, row := range rows {
		key := shareKey(row.PrincipalType, row.PrincipalID)
		agg, ok := aggregates[key]
		if !ok {
			agg = &shareAggregate{
				PrincipalType: row.PrincipalType,
				PrincipalID:   row.PrincipalID,
				Permissions:   make(map[string]struct{}),
				GrantedByIDs:  make(map[string]struct{}),
			}
			aggregates[key] = agg
		}

		if row.PermissionID != "" {
			agg.Permissions[row.PermissionID] = struct{}{}
		}

		if row.ExpiresAt != nil {
			if agg.ExpiresAt == nil || row.ExpiresAt.Before(*agg.ExpiresAt) {
				exp := *row.ExpiresAt
				agg.ExpiresAt = &exp
			}
		}

		if len(row.Metadata) > 0 && agg.Metadata == nil {
			if decoded := decodeMetadata(row.Metadata); len(decoded) > 0 {
				agg.Metadata = decoded
			}
		}

		if row.GrantedByID != nil && *row.GrantedByID != "" {
			agg.GrantedByIDs[*row.GrantedByID] = struct{}{}
			grantorIDs[*row.GrantedByID] = struct{}{}
		}

		switch row.PrincipalType {
		case PrincipalTypeUser:
			userIDs[row.PrincipalID] = struct{}{}
		case PrincipalTypeTeam:
			teamIDs[row.PrincipalID] = struct{}{}
		}
	}

	userActors, err := loadUserActors(ctx, db, setKeys(userIDs))
	if err != nil {
		return nil, err
	}

	teamActors, err := loadTeamActors(ctx, db, setKeys(teamIDs))
	if err != nil {
		return nil, err
	}

	grantorActors, err := loadUserActors(ctx, db, setKeys(grantorIDs))
	if err != nil {
		return nil, err
	}

	dtos := make([]ConnectionShareDTO, 0, len(aggregates))
	for _, agg := range aggregates {
		shareID := shareKey(agg.PrincipalType, agg.PrincipalID)

		var principal ShareActor
		switch agg.PrincipalType {
		case PrincipalTypeUser:
			principal = userActors[agg.PrincipalID]
		case PrincipalTypeTeam:
			principal = teamActors[agg.PrincipalID]
		default:
			principal = ShareActor{
				ID:   agg.PrincipalID,
				Type: agg.PrincipalType,
				Name: agg.PrincipalID,
			}
		}

		var grantedBy *ShareActor
		for id := range agg.GrantedByIDs {
			if actor, ok := grantorActors[id]; ok {
				copy := actor
				grantedBy = &copy
				break
			}
		}

		dtos = append(dtos, ConnectionShareDTO{
			ShareID:          shareID,
			Principal:        principal,
			PermissionScopes: sortedKeys(agg.Permissions),
			ExpiresAt:        agg.ExpiresAt,
			GrantedBy:        grantedBy,
			Metadata:         agg.Metadata,
		})
	}

	return dtos, nil
}

func loadUserActors(ctx context.Context, db *gorm.DB, ids []string) (map[string]ShareActor, error) {
	actors := make(map[string]ShareActor, len(ids))
	if len(ids) == 0 {
		return actors, nil
	}

	var users []models.User
	if err := db.WithContext(ctx).
		Select("id", "username", "email", "first_name", "last_name").
		Where("id IN ?", ids).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("connection share service: load users: %w", err)
	}

	for _, user := range users {
		actors[user.ID] = ShareActor{
			ID:    user.ID,
			Type:  PrincipalTypeUser,
			Name:  composeUserDisplayName(&user),
			Email: user.Email,
		}
	}
	return actors, nil
}

func loadTeamActors(ctx context.Context, db *gorm.DB, ids []string) (map[string]ShareActor, error) {
	actors := make(map[string]ShareActor, len(ids))
	if len(ids) == 0 {
		return actors, nil
	}

	var teams []models.Team
	if err := db.WithContext(ctx).
		Select("id", "name", "description").
		Where("id IN ?", ids).
		Find(&teams).Error; err != nil {
		return nil, fmt.Errorf("connection share service: load teams: %w", err)
	}

	for _, team := range teams {
		actors[team.ID] = ShareActor{
			ID:   team.ID,
			Type: PrincipalTypeTeam,
			Name: team.Name,
		}
	}
	return actors, nil
}

func shareKey(principalType, principalID string) string {
	return fmt.Sprintf("%s:%s", principalType, principalID)
}

func parseShareID(value string) (string, string, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("invalid share id")
	}
	return parts[0], parts[1], nil
}

func decodeMetadata(raw datatypes.JSON) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}
	return data
}

func composeUserDisplayName(user *models.User) string {
	if user == nil {
		return ""
	}

	first := strings.TrimSpace(user.FirstName)
	last := strings.TrimSpace(user.LastName)
	switch {
	case first != "" && last != "":
		return strings.TrimSpace(first + " " + last)
	case first != "":
		return first
	case last != "":
		return last
	case user.Username != "":
		return user.Username
	case user.Email != "":
		return user.Email
	default:
		return user.ID
	}
}

func setKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	return keys
}

func sortedKeys(set map[string]struct{}) []string {
	keys := setKeys(set)
	sort.Strings(keys)
	return keys
}
