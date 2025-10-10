package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

var (
	// ErrTeamNotFound indicates the requested team does not exist.
	ErrTeamNotFound = apperrors.New("TEAM_NOT_FOUND", "Team not found", http.StatusNotFound)
	// ErrTeamMemberAlreadyExists signals the user is already a member of the team.
	ErrTeamMemberAlreadyExists = apperrors.New("TEAM_MEMBER_EXISTS", "User already assigned to team", http.StatusConflict)
	// ErrTeamMemberNotFound indicates the requested membership does not exist.
	ErrTeamMemberNotFound = apperrors.New("TEAM_MEMBER_NOT_FOUND", "User is not a member of the team", http.StatusNotFound)
)

// CreateTeamInput captures new team metadata.
type CreateTeamInput struct {
	Name        string
	Description string
}

// UpdateTeamInput describes mutable team fields.
type UpdateTeamInput struct {
	Name        *string
	Description *string
}

// TeamService handles team lifecycle and membership management.
type TeamService struct {
	db           *gorm.DB
	auditService *AuditService
}

// NewTeamService constructs a TeamService instance.
func NewTeamService(db *gorm.DB, auditService *AuditService) (*TeamService, error) {
	if db == nil {
		return nil, errors.New("team service: db is required")
	}
	return &TeamService{
		db:           db,
		auditService: auditService,
	}, nil
}

// Create registers a new team.
func (s *TeamService) Create(ctx context.Context, input CreateTeamInput) (*models.Team, error) {
	ctx = ensureContext(ctx)

	name := strings.TrimSpace(input.Name)

	if name == "" {
		return nil, apperrors.NewBadRequest("team name is required")
	}

	team := &models.Team{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
	}

	if err := s.db.WithContext(ctx).Create(team).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("team name already exists")
		}
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "team.create",
		Resource: team.ID,
		Result:   "success",
		Metadata: map[string]any{
			"name": team.Name,
		},
	})

	return team, nil
}

// Update modifies team metadata.
func (s *TeamService) Update(ctx context.Context, id string, input UpdateTeamInput) (*models.Team, error) {
	ctx = ensureContext(ctx)

	var team models.Team
	err := s.db.WithContext(ctx).First(&team, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("team service: load team: %w", err)
	}

	updates := map[string]any{}
	if input.Name != nil {
		if name := strings.TrimSpace(*input.Name); name != "" && name != team.Name {
			updates["name"] = name
		}
	}
	if input.Description != nil {
		updates["description"] = strings.TrimSpace(*input.Description)
	}

	if len(updates) == 0 {
		return &team, nil
	}

	if err := s.db.WithContext(ctx).Model(&team).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("team name already exists")
		}
		return nil, fmt.Errorf("team service: update team: %w", err)
	}

	if err := s.db.WithContext(ctx).First(&team, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("team service: reload team: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "team.update",
		Resource: team.ID,
		Result:   "success",
		Metadata: updates,
	})

	return &team, nil
}

// GetByID loads a team with related membership.
func (s *TeamService) GetByID(ctx context.Context, id string) (*models.Team, error) {
	ctx = ensureContext(ctx)

	var team models.Team
	err := s.db.WithContext(ctx).
		Preload("Users").
		First(&team, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("team service: get team: %w", err)
	}
	return &team, nil
}

// List returns all teams.
func (s *TeamService) List(ctx context.Context) ([]models.Team, error) {
	ctx = ensureContext(ctx)

	var teams []models.Team
	if err := s.db.WithContext(ctx).
		Order("created_at ASC").
		Find(&teams).Error; err != nil {
		return nil, fmt.Errorf("team service: list teams: %w", err)
	}

	return teams, nil
}

// Delete removes a team by identifier.
func (s *TeamService) Delete(ctx context.Context, id string) error {
	ctx = ensureContext(ctx)

	var team models.Team
	err := s.db.WithContext(ctx).First(&team, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTeamNotFound
	}
	if err != nil {
		return fmt.Errorf("team service: load team: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&team).Error; err != nil {
		return fmt.Errorf("team service: delete team: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "team.delete",
		Resource: team.ID,
		Result:   "success",
	})

	return nil
}

// AddMember attaches a user to a team.
func (s *TeamService) AddMember(ctx context.Context, teamID, userID string) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(teamID) == "" || strings.TrimSpace(userID) == "" {
		return apperrors.NewBadRequest("team id and user id are required")
	}

	var team models.Team
	if err := s.db.WithContext(ctx).First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTeamNotFound
		}
		return fmt.Errorf("team service: load team: %w", err)
	}

	var user models.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("team service: load user: %w", err)
	}

	var existing int64
	if err := s.db.WithContext(ctx).
		Table("user_teams").
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Count(&existing).Error; err != nil {
		return fmt.Errorf("team service: check membership: %w", err)
	}
	if existing > 0 {
		return ErrTeamMemberAlreadyExists
	}

	if err := s.db.WithContext(ctx).Model(&team).Association("Users").Append(&user); err != nil {
		return fmt.Errorf("team service: append member: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "team.add_member",
		Resource: teamID,
		Result:   "success",
		Metadata: map[string]any{"user_id": userID},
	})

	return nil
}

// RemoveMember detaches a user from a team.
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID string) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(teamID) == "" || strings.TrimSpace(userID) == "" {
		return apperrors.NewBadRequest("team id and user id are required")
	}

	var team models.Team
	if err := s.db.WithContext(ctx).First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTeamNotFound
		}
		return fmt.Errorf("team service: load team: %w", err)
	}

	var user models.User
	if err := s.db.WithContext(ctx).First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("team service: load user: %w", err)
	}

	var existing int64
	if err := s.db.WithContext(ctx).
		Table("user_teams").
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Count(&existing).Error; err != nil {
		return fmt.Errorf("team service: check membership: %w", err)
	}
	if existing == 0 {
		return ErrTeamMemberNotFound
	}

	if err := s.db.WithContext(ctx).Model(&team).Association("Users").Delete(&user); err != nil {
		return fmt.Errorf("team service: remove member: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "team.remove_member",
		Resource: teamID,
		Result:   "success",
		Metadata: map[string]any{"user_id": userID},
	})

	return nil
}

// ListMembers returns the users assigned to a team.
func (s *TeamService) ListMembers(ctx context.Context, teamID string) ([]models.User, error) {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(teamID) == "" {
		return nil, apperrors.NewBadRequest("team id is required")
	}

	var team models.Team
	if err := s.db.WithContext(ctx).
		Preload("Users").
		First(&team, "id = ?", teamID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("team service: load team: %w", err)
	}

	return team.Users, nil
}
