package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

var (
	// ErrRoleNotFound indicates the requested role does not exist.
	ErrRoleNotFound = apperrors.New("ROLE_NOT_FOUND", "Role not found", http.StatusNotFound)
	// ErrSystemRoleImmutable prevents destructive operations on system roles.
	ErrSystemRoleImmutable = apperrors.New("ROLE_IMMUTABLE", "System roles cannot be modified", http.StatusBadRequest)
)

// PermissionService provides role management and permission assignment helpers.
type PermissionService struct {
	db           *gorm.DB
	auditService *AuditService
}

// NewPermissionService constructs a PermissionService using the provided database handle.
func NewPermissionService(db *gorm.DB, audit *AuditService) (*PermissionService, error) {
	if db == nil {
		return nil, errors.New("permission service: db is required")
	}
	return &PermissionService{
		db:           db,
		auditService: audit,
	}, nil
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
		return nil, apperrors.NewBadRequest("role name is required")
	}

	role := &models.Role{
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		IsSystem:    input.IsSystem,
	}

	if err := s.db.WithContext(ctx).Create(role).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("role name already exists")
		}
		return nil, fmt.Errorf("permission service: create role: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "role.create",
		Resource: role.ID,
		Result:   "success",
		Metadata: map[string]any{
			"name":      role.Name,
			"is_system": role.IsSystem,
		},
	})

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
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("role name already exists")
		}
		return nil, fmt.Errorf("permission service: update role: %w", err)
	}

	if err := s.db.WithContext(ctx).First(&role, "id = ?", roleID).Error; err != nil {
		return nil, fmt.Errorf("permission service: reload role: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "role.update",
		Resource: role.ID,
		Result:   "success",
		Metadata: updates,
	})

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

	if err := s.db.WithContext(ctx).Model(&role).Association("Permissions").Clear(); err != nil {
		return fmt.Errorf("permission service: clear role permissions: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&role).Association("Users").Clear(); err != nil {
		return fmt.Errorf("permission service: clear role users: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&role).Error; err != nil {
		return fmt.Errorf("permission service: delete role: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "role.delete",
		Resource: role.ID,
		Result:   "success",
		Metadata: map[string]any{
			"name": role.Name,
		},
	})

	return nil
}

// ListRoles returns all roles ordered by creation date.
func (s *PermissionService) ListRoles(ctx context.Context) ([]models.Role, error) {
	ctx = ensureContext(ctx)

	var roles []models.Role
	if err := s.db.WithContext(ctx).Preload("Permissions").Order("created_at ASC").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("permission service: list roles: %w", err)
	}
	return roles, nil
}

// SetRolePermissions replaces the role's permissions with the provided set, ensuring dependencies are included.
func (s *PermissionService) SetRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	ctx = ensureContext(ctx)

	var applied []string

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
			applied = []string{}
			return tx.Model(&role).Association("Permissions").Clear()
		}

		ids := make([]string, 0, len(finalSet))
		for id := range finalSet {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		applied = ids

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
	if err != nil {
		return err
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "role.set_permissions",
		Resource: roleID,
		Result:   "success",
		Metadata: map[string]any{
			"permission_ids": applied,
		},
	})

	return nil
}

func expandWithDependencies(permissionIDs []string) (map[string]struct{}, error) {
	final := make(map[string]struct{})

	for _, id := range permissionIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := permissions.Get(id); !ok {
			return nil, apperrors.NewBadRequest(fmt.Sprintf("%s %q", permissions.ErrUnknownPermission.Error(), id))
		}

		final[id] = struct{}{}

		deps, err := permissions.ResolveDependencies(id)
		if err != nil {
			return nil, apperrors.NewBadRequest(err.Error())
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
