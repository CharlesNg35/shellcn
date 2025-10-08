package permissions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

// Checker evaluates user permissions against the registry.
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

	var user models.User
	if err := c.db.WithContext(ctx).
		Preload("Roles.Permissions").
		First(&user, "id = ?", userID).Error; err != nil {
		return false, fmt.Errorf("permission checker: load user: %w", err)
	}

	if user.IsRoot {
		return true, nil
	}

	if _, ok := Get(permissionID); !ok {
		return false, fmt.Errorf("%w %q", ErrUnknownPermission, permissionID)
	}

	userPerms, err := collectUserPermissions(&user)
	if err != nil {
		return false, err
	}

	dependencies, err := ResolveDependencies(permissionID)
	if err != nil {
		return false, err
	}

	for _, dep := range dependencies {
		if _, ok := userPerms[dep]; !ok {
			return false, nil
		}
	}

	_, ok := userPerms[permissionID]
	return ok, nil
}

// GetUserPermissions returns the distinct permission IDs granted to the user.
func (c *Checker) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	ctx = ensureContext(ctx)

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("permission checker: user id is required")
	}

	var user models.User
	if err := c.db.WithContext(ctx).
		Preload("Roles.Permissions").
		First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("permission checker: load user: %w", err)
	}

	if user.IsRoot {
		perms := GetAll()
		ids := make([]string, 0, len(perms))
		for id := range perms {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids, nil
	}

	userPerms, err := collectUserPermissions(&user)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(userPerms))
	for id := range userPerms {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func collectUserPermissions(user *models.User) (map[string]struct{}, error) {
	granted := make([]string, 0)
	for _, role := range user.Roles {
		for _, perm := range role.Permissions {
			granted = append(granted, perm.ID)
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
