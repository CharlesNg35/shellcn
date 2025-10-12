package permissions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

const (
	principalTypeUser = "user"
	principalTypeTeam = "team"
)

// Checker evaluates user permissions against the registry and persisted grants.
type Checker struct {
	db *gorm.DB
}

// NewChecker constructs a permission checker backed by the provided database.
func NewChecker(db *gorm.DB) (*Checker, error) {
	if db == nil {
		return nil, errors.New("permission checker: db is required")
	}
	return &Checker{db: db}, nil
}

// Check determines whether the user has the specified permission, considering dependencies.
func (c *Checker) Check(ctx context.Context, userID, permissionID string) (bool, error) {
	ctx = ensureContext(ctx)

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, errors.New("permission checker: user id is required")
	}
	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return false, errors.New("permission checker: permission id is required")
	}

	grants, err := c.loadUserGrants(ctx, userID)
	if err != nil {
		return false, err
	}

	if grants.IsRoot {
		return true, nil
	}

	return hasPermission(grants.Permissions, permissionID)
}

// CheckResource resolves whether the user has the requested permission for a specific resource.
func (c *Checker) CheckResource(ctx context.Context, userID, resourceType, resourceID, permissionID string) (bool, error) {
	ctx = ensureContext(ctx)

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, errors.New("permission checker: user id is required")
	}
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" {
		return false, errors.New("permission checker: resource type is required")
	}
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return false, errors.New("permission checker: resource id is required")
	}
	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return false, errors.New("permission checker: permission id is required")
	}

	grants, err := c.loadUserGrants(ctx, userID)
	if err != nil {
		return false, err
	}

	if grants.IsRoot {
		return true, nil
	}

	resourcePerms, err := c.loadResourcePermissions(ctx, grants, resourceType, resourceID)
	if err != nil {
		return false, err
	}

	effective := mergePermissionSets(grants.Permissions, resourcePerms)
	return hasPermission(effective, permissionID)
}

// GetUserPermissions returns the distinct permission IDs granted to the user.
func (c *Checker) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	ctx = ensureContext(ctx)

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("permission checker: user id is required")
	}

	grants, err := c.loadUserGrants(ctx, userID)
	if err != nil {
		return nil, err
	}

	if grants.IsRoot {
		perms := GetAll()
		ids := make([]string, 0, len(perms))
		for id := range perms {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids, nil
	}

	ids := make([]string, 0, len(grants.Permissions))
	for id := range grants.Permissions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

type userGrants struct {
	ID          string
	IsRoot      bool
	TeamIDs     []string
	Permissions map[string]struct{}
}

func (c *Checker) loadUserGrants(ctx context.Context, userID string) (*userGrants, error) {
	var user models.User
	if err := c.db.WithContext(ctx).
		Preload("Roles.Permissions").
		Preload("Teams.Roles.Permissions").
		First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("permission checker: load user: %w", err)
	}

	teamIDs := make([]string, 0, len(user.Teams))
	for _, team := range user.Teams {
		teamIDs = append(teamIDs, team.ID)
	}

	requiresVerification, err := c.requiresEmailVerification(ctx, user.AuthProvider)
	if err != nil {
		return nil, err
	}
	if requiresVerification {
		verified, verr := c.isEmailVerified(ctx, user.ID)
		if verr != nil {
			return nil, verr
		}
		if !verified {
			return &userGrants{
				ID:          user.ID,
				IsRoot:      false,
				TeamIDs:     teamIDs,
				Permissions: map[string]struct{}{},
			}, nil
		}
	}

	if user.IsRoot {
		return &userGrants{
			ID:      user.ID,
			IsRoot:  true,
			TeamIDs: teamIDs,
		}, nil
	}

	perms, err := collectUserPermissions(&user)
	if err != nil {
		return nil, err
	}

	return &userGrants{
		ID:          user.ID,
		IsRoot:      false,
		TeamIDs:     teamIDs,
		Permissions: perms,
	}, nil
}

func (c *Checker) loadResourcePermissions(ctx context.Context, grants *userGrants, resourceType, resourceID string) (map[string]struct{}, error) {
	query := c.db.WithContext(ctx).
		Model(&models.ResourcePermission{}).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC())

	clauses := []string{"(principal_type = ? AND principal_id = ?)"}
	args := []any{principalTypeUser, grants.ID}

	if len(grants.TeamIDs) > 0 {
		clauses = append(clauses, "(principal_type = ? AND principal_id IN ?)")
		args = append(args, principalTypeTeam, grants.TeamIDs)
	}

	query = query.Where(strings.Join(clauses, " OR "), args...)

	var rows []models.ResourcePermission
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("permission checker: load resource permissions: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	granted := make([]string, 0, len(rows))
	for _, row := range rows {
		granted = append(granted, row.PermissionID)
	}

	return expandImplied(granted)
}

func (c *Checker) requiresEmailVerification(ctx context.Context, provider string) (bool, error) {
	if normalizeAuthProvider(provider) != "local" {
		return false, nil
	}

	var record models.AuthProvider
	if err := c.db.WithContext(ctx).
		Select("require_email_verification").
		Where("type = ?", "local").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		return false, fmt.Errorf("permission checker: load local provider: %w", err)
	}

	return record.RequireEmailVerification, nil
}

func (c *Checker) isEmailVerified(ctx context.Context, userID string) (bool, error) {
	var verification models.EmailVerification
	if err := c.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&verification).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		if strings.Contains(err.Error(), "no such table") {
			return true, nil
		}
		return false, fmt.Errorf("permission checker: load email verification: %w", err)
	}

	return verification.VerifiedAt != nil, nil
}

func normalizeAuthProvider(provider string) string {
	value := strings.ToLower(strings.TrimSpace(provider))
	if value == "" {
		return "local"
	}
	return value
}

func hasPermission(perms map[string]struct{}, permissionID string) (bool, error) {
	if _, ok := Get(permissionID); !ok {
		return false, fmt.Errorf("%w %q", ErrUnknownPermission, permissionID)
	}

	dependencies, err := ResolveDependencies(permissionID)
	if err != nil {
		return false, err
	}

	for _, dep := range dependencies {
		if _, ok := perms[dep]; !ok {
			return false, nil
		}
	}

	_, ok := perms[permissionID]
	return ok, nil
}

func mergePermissionSets(sets ...map[string]struct{}) map[string]struct{} {
	merged := make(map[string]struct{})
	for _, set := range sets {
		for id := range set {
			merged[id] = struct{}{}
		}
	}
	return merged
}

func collectUserPermissions(user *models.User) (map[string]struct{}, error) {
	granted := make([]string, 0)
	for _, role := range user.Roles {
		for _, perm := range role.Permissions {
			granted = append(granted, perm.ID)
		}
	}
	for _, team := range user.Teams {
		for _, role := range team.Roles {
			for _, perm := range role.Permissions {
				granted = append(granted, perm.ID)
			}
		}
	}

	return expandImplied(granted)
}

func expandImplied(ids []string) (map[string]struct{}, error) {
	perms := make(map[string]struct{})

	var visit func(string) error
	visit = func(id string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		if _, exists := perms[id]; exists {
			return nil
		}

		def, ok := Get(id)
		if !ok {
			return fmt.Errorf("%w %q", ErrUnknownPermission, id)
		}

		perms[id] = struct{}{}
		for _, implied := range def.Implies {
			if err := visit(implied); err != nil {
				return err
			}
		}
		return nil
	}

	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}

	return perms, nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
