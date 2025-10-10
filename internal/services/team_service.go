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
	checker      PermissionChecker
}

// NewTeamService constructs a TeamService instance.
func NewTeamService(db *gorm.DB, auditService *AuditService, checker PermissionChecker) (*TeamService, error) {
	if db == nil {
		return nil, errors.New("team service: db is required")
	}
	return &TeamService{
		db:           db,
		auditService: auditService,
		checker:      checker,
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

// GetByID loads a team with related membership for the requesting user.
func (s *TeamService) GetByID(ctx context.Context, id, requesterID string) (*models.Team, error) {
	ctx = ensureContext(ctx)

	var team models.Team
	err := s.db.WithContext(ctx).
		Preload("Users.Roles").
		Preload("Roles").
		First(&team, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("team service: get team: %w", err)
	}

	requesterID = strings.TrimSpace(requesterID)
	if requesterID == "" {
		return &team, nil
	}

	userCtx, err := s.userContext(ctx, requesterID)
	if err != nil {
		return nil, err
	}

	if userCtx.IsRoot {
		return &team, nil
	}

	canManage, err := s.canManageTeams(ctx, requesterID)
	if err != nil {
		return nil, err
	}
	if canManage {
		return &team, nil
	}

	if containsString(userCtx.TeamIDs, id) {
		return &team, nil
	}

	canView, err := s.canViewTeams(ctx, requesterID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, apperrors.ErrForbidden
	}

	return &team, nil
}

// List returns teams visible to the requesting user.
func (s *TeamService) List(ctx context.Context, requesterID string) ([]models.Team, error) {
	ctx = ensureContext(ctx)

	requesterID = strings.TrimSpace(requesterID)
	var userCtx teamUserContext
	var err error
	if requesterID != "" {
		userCtx, err = s.userContext(ctx, requesterID)
		if err != nil {
			return nil, err
		}
	}

	canManage := false
	if requesterID == "" {
		canManage = true
	} else if userCtx.IsRoot {
		canManage = true
	} else {
		canManage, err = s.canManageTeams(ctx, requesterID)
		if err != nil {
			return nil, err
		}
	}

	query := s.db.WithContext(ctx).
		Preload("Users").
		Preload("Roles").
		Order("created_at ASC")

	if !canManage {
		if len(userCtx.TeamIDs) == 0 {
			return []models.Team{}, nil
		}
		query = query.Where("id IN ?", userCtx.TeamIDs)
	}

	var teams []models.Team
	if err := query.Find(&teams).Error; err != nil {
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
func (s *TeamService) ListMembers(ctx context.Context, requesterID, teamID string) ([]models.User, error) {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(teamID) == "" {
		return nil, apperrors.NewBadRequest("team id is required")
	}

	team, err := s.GetByID(ctx, teamID, requesterID)
	if err != nil {
		return nil, err
	}

	return team.Users, nil
}

// ListRoles returns roles assigned to the team.
func (s *TeamService) ListRoles(ctx context.Context, requesterID, teamID string) ([]models.Role, error) {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(teamID) == "" {
		return nil, apperrors.NewBadRequest("team id is required")
	}

	team, err := s.GetByID(ctx, teamID, requesterID)
	if err != nil {
		return nil, err
	}

	return team.Roles, nil
}

// SetRoles replaces the team's role assignments.
func (s *TeamService) SetRoles(ctx context.Context, teamID string, roleIDs []string) ([]models.Role, error) {
	ctx = ensureContext(ctx)

	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, apperrors.NewBadRequest("team id is required")
	}

	cleanIDs := normaliseIDs(roleIDs)

	var result []models.Role
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var team models.Team
		if err := tx.Preload("Roles").First(&team, "id = ?", teamID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTeamNotFound
			}
			return fmt.Errorf("team service: load team: %w", err)
		}

		var roles []models.Role
		if len(cleanIDs) > 0 {
			if err := tx.Where("id IN ?", cleanIDs).Find(&roles).Error; err != nil {
				return fmt.Errorf("team service: load roles: %w", err)
			}
			if len(roles) != len(cleanIDs) {
				return apperrors.NewBadRequest("one or more roles were not found")
			}
		}

		if err := tx.Model(&team).Association("Roles").Replace(roles); err != nil {
			return fmt.Errorf("team service: replace roles: %w", err)
		}

		if err := tx.Preload("Roles").First(&team, "id = ?", teamID).Error; err != nil {
			return fmt.Errorf("team service: reload team: %w", err)
		}

		result = team.Roles

		recordAudit(s.auditService, ctx, AuditEntry{
			Action:   "team.set_roles",
			Resource: team.ID,
			Result:   "success",
			Metadata: map[string]any{
				"role_ids": cleanIDs,
			},
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *TeamService) canViewTeams(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return true, nil
	}
	if s.checker == nil {
		return true, nil
	}
	return s.checker.Check(ctx, userID, "team.view")
}

func (s *TeamService) canManageTeams(ctx context.Context, userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return true, nil
	}
	if s.checker == nil {
		return true, nil
	}
	if ok, err := s.checker.Check(ctx, userID, "team.manage"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}
	return s.checker.Check(ctx, userID, "permission.manage")
}

func (s *TeamService) userContext(ctx context.Context, userID string) (teamUserContext, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return teamUserContext{}, errors.New("team service: user id is required")
	}

	var user models.User
	if err := s.db.WithContext(ctx).
		Preload("Teams").
		First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return teamUserContext{}, apperrors.ErrNotFound
		}
		return teamUserContext{}, fmt.Errorf("team service: load user context: %w", err)
	}

	teamIDs := make([]string, 0, len(user.Teams))
	for _, team := range user.Teams {
		teamIDs = append(teamIDs, team.ID)
	}

	return teamUserContext{
		ID:      user.ID,
		IsRoot:  user.IsRoot,
		TeamIDs: teamIDs,
	}, nil
}

type teamUserContext struct {
	ID      string
	IsRoot  bool
	TeamIDs []string
}
