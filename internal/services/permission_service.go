package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
)

var (
	// ErrRoleNotFound indicates the requested role does not exist.
	ErrRoleNotFound = errors.New("permission service: role not found")
	// ErrSystemRoleImmutable prevents destructive operations on system roles.
	ErrSystemRoleImmutable = errors.New("permission service: system roles are immutable")
)

// PermissionService provides role management and permission assignment helpers.
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService constructs a PermissionService using the provided database handle.
func NewPermissionService(db *gorm.DB) (*PermissionService, error) {
	if db == nil {
		return nil, errors.New("permission service: db is required")
	}
	return &PermissionService{db: db}, nil
}

// CreateRoleInput describes the payload accepted by CreateRole.
type CreateRoleInput struct {
	Name        string
	Description string
	IsSystem    bool
}

// UpdateRoleInput describes mutable fields on a role.
type UpdateRoleInput struct {
	Name        string
	Description string
}

// CreateRole registers a new role.
func (s *PermissionService) CreateRole(ctx context.Context, input CreateRoleInput) (*models.Role, error) {
	ctx = ensureContext(ctx)

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("permission service: role name is required")
	}

	role := &models.Role{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		IsSystem:    input.IsSystem,
	}

	if err := s.db.WithContext(ctx).Create(role).Error; err != nil {
		return nil, fmt.Errorf("permission service: create role: %w", err)
	}

	return role, nil
}

// UpdateRole modifies existing role metadata.
func (s *PermissionService) UpdateRole(ctx context.Context, roleID string, input UpdateRoleInput) (*models.Role, error) {
	ctx = ensureContext(ctx)

	var role models.Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("permission service: load role: %w", err)
	}

	if role.IsSystem {
		if strings.TrimSpace(input.Name) != "" && input.Name != role.Name {
			return nil, ErrSystemRoleImmutable
		}
	}

	updates := map[string]any{}
	if name := strings.TrimSpace(input.Name); name != "" && name != role.Name {
		updates["name"] = name
	}
	if desc := strings.TrimSpace(input.Description); desc != role.Description {
		updates["description"] = desc
	}

	if len(updates) == 0 {
		return &role, nil
	}

	if err := s.db.WithContext(ctx).Model(&role).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("permission service: update role: %w", err)
	}

	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return nil, fmt.Errorf("permission service: reload role: %w", err)
	}

	return &role, nil
}

// DeleteRole removes non-system roles permanently.
func (s *PermissionService) DeleteRole(ctx context.Context, roleID string) error {
	ctx = ensureContext(ctx)

	var role models.Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("permission service: load role: %w", err)
	}

	if role.IsSystem {
		return ErrSystemRoleImmutable
	}

	if err := s.db.WithContext(ctx).Delete(&role).Error; err != nil {
		return fmt.Errorf("permission service: delete role: %w", err)
	}

	return nil
}

// ListRoles returns all roles ordered by creation date.
func (s *PermissionService) ListRoles(ctx context.Context) ([]models.Role, error) {
	ctx = ensureContext(ctx)

	var roles []models.Role
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("permission service: list roles: %w", err)
	}
	return roles, nil
}

// SetRolePermissions replaces the role's permissions with the provided set, ensuring dependencies are included.
func (s *PermissionService) SetRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	ctx = ensureContext(ctx)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role models.Role
		if err := tx.First(&role, "id = ?", roleID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleNotFound
			}
			return fmt.Errorf("permission service: load role: %w", err)
		}

		if role.IsSystem {
			return ErrSystemRoleImmutable
		}

		finalSet, err := expandWithDependencies(permissionIDs)
		if err != nil {
			return err
		}

		if len(finalSet) == 0 {
			return tx.Model(&role).Association("Permissions").Clear()
		}

		ids := make([]string, 0, len(finalSet))
		for id := range finalSet {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		var perms []models.Permission
		if err := tx.Where("id IN ?", ids).Find(&perms).Error; err != nil {
			return fmt.Errorf("permission service: load permissions: %w", err)
		}
		if len(perms) != len(ids) {
			return fmt.Errorf("permission service: some permissions are missing in database")
		}

		if err := tx.Model(&role).Association("Permissions").Replace(perms); err != nil {
			return fmt.Errorf("permission service: update permissions: %w", err)
		}

		return nil
	})
}

func expandWithDependencies(permissionIDs []string) (map[string]struct{}, error) {
	final := make(map[string]struct{})

	for _, id := range permissionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := permissions.Get(id); !ok {
			return nil, fmt.Errorf("%w %q", permissions.ErrUnknownPermission, id)
		}

		final[id] = struct{}{}

		deps, err := permissions.ResolveDependencies(id)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
			final[dep] = struct{}{}
		}
	}

	return final, nil
}

// ListUserPermissions resolves permissions granted to the supplied user.
func (s *PermissionService) ListUserPermissions(ctx context.Context, userID string) ([]string, error) {
	ctx = ensureContext(ctx)
	checker, err := permissions.NewChecker(s.db)
	if err != nil {
		return nil, err
	}
	perms, err := checker.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	return perms, nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
