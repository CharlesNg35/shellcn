package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

var (
	// ErrUserNotFound indicates the requested user does not exist.
	ErrUserNotFound = apperrors.New("USER_NOT_FOUND", "User not found", http.StatusNotFound)
	// ErrRootUserImmutable ensures the root account cannot be deactivated or deleted.
	ErrRootUserImmutable = apperrors.New("USER_ROOT_IMMUTABLE", "Root user cannot perform this operation", http.StatusBadRequest)
)

// CreateUserInput describes the fields accepted when creating a user.
type CreateUserInput struct {
	Username  string
	Email     string
	Password  string
	FirstName string
	LastName  string
	Avatar    string
	IsRoot    bool
	IsActive  *bool
}

// UpdateUserInput enumerates mutable user attributes.
type UpdateUserInput struct {
	Username  *string
	Email     *string
	FirstName *string
	LastName  *string
	Avatar    *string
}

// UserFilters captures listing filters.
type UserFilters struct {
	IsActive *bool
	Query    string
}

// ListUsersOptions controls pagination for user listing.
type ListUsersOptions struct {
	Page     int
	PageSize int
	Filters  UserFilters
}

// UserService manages CRUD lifecycle for users including activation and password management.
type UserService struct {
	db           *gorm.DB
	auditService *AuditService
}

func isExternalProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p != "" && p != "local"
}

// NewUserService constructs a UserService instance.
func NewUserService(db *gorm.DB, auditService *AuditService) (*UserService, error) {
	if db == nil {
		return nil, errors.New("user service: db is required")
	}
	return &UserService{
		db:           db,
		auditService: auditService,
	}, nil
}

// Create provisions a new user with a hashed password.
func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*models.User, error) {
	ctx = ensureContext(ctx)

	username := strings.TrimSpace(input.Username)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if username == "" {
		return nil, apperrors.NewBadRequest("username is required")
	}
	if email == "" {
		return nil, apperrors.NewBadRequest("email is required")
	}
	if strings.TrimSpace(input.Password) == "" {
		return nil, apperrors.NewBadRequest("password is required")
	}

	hashed, err := crypto.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("user service: hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		Password:     hashed,
		FirstName:    strings.TrimSpace(input.FirstName),
		LastName:     strings.TrimSpace(input.LastName),
		Avatar:       strings.TrimSpace(input.Avatar),
		IsRoot:       input.IsRoot,
		IsActive:     true,
		AuthProvider: "local",
	}

	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		defaultRoleIDs := []string{"user"}
		if user.IsRoot {
			defaultRoleIDs = append(defaultRoleIDs, "admin")
		}

		var roles []models.Role
		if err := tx.Where("id IN ?", defaultRoleIDs).Find(&roles).Error; err != nil {
			return fmt.Errorf("user service: load default roles: %w", err)
		}
		if len(roles) != len(defaultRoleIDs) {
			return fmt.Errorf("user service: default roles missing: expected %d, found %d", len(defaultRoleIDs), len(roles))
		}

		roleInterfaces := make([]any, len(roles))
		for i := range roles {
			roleInterfaces[i] = &roles[i]
		}
		if err := tx.Model(user).Association("Roles").Append(roleInterfaces...); err != nil {
			return fmt.Errorf("user service: assign default roles: %w", err)
		}

		return nil
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("username or email already exists")
		}
		return nil, fmt.Errorf("user service: create user: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "user.create",
		Resource: user.ID,
		Result:   "success",
		Metadata: map[string]any{
			"username": user.Username,
			"email":    user.Email,
			"is_root":  user.IsRoot,
		},
	})

	return user, nil
}

// GetByID loads a user by identifier including associations.
func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	ctx = ensureContext(ctx)

	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Teams").
		Preload("Roles.Permissions").
		First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user service: get user: %w", err)
	}
	return &user, nil
}

// List retrieves users matching the supplied filters with pagination.
func (s *UserService) List(ctx context.Context, opts ListUsersOptions) ([]models.User, int64, error) {
	ctx = ensureContext(ctx)

	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PageSize
	if perPage <= 0 || perPage > 200 {
		perPage = 50
	}

	query := s.db.WithContext(ctx).Model(&models.User{})
	if opts.Filters.IsActive != nil {
		query = query.Where("is_active = ?", *opts.Filters.IsActive)
	}
	if q := strings.TrimSpace(opts.Filters.Query); q != "" {
		pattern := "%" + strings.ToLower(q) + "%"
		query = query.Where("LOWER(username) LIKE ? OR LOWER(email) LIKE ?", pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("user service: count users: %w", err)
	}

	var users []models.User
	if err := query.
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Preload("Roles.Permissions").
		Preload("Teams").
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("user service: list users: %w", err)
	}

	return users, total, nil
}

// Update persists mutable attributes for an existing user.
func (s *UserService) Update(ctx context.Context, id string, input UpdateUserInput) (*models.User, error) {
	ctx = ensureContext(ctx)

	var user models.User
	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user service: load user: %w", err)
	}

	if isExternalProvider(user.AuthProvider) {
		if input.Username != nil {
			trimmed := strings.TrimSpace(*input.Username)
			if trimmed != "" && trimmed != user.Username {
				return nil, apperrors.NewBadRequest("user profile is managed by an external provider")
			}
		}
		if input.Email != nil {
			trimmed := strings.ToLower(strings.TrimSpace(*input.Email))
			if trimmed != "" && trimmed != strings.ToLower(user.Email) {
				return nil, apperrors.NewBadRequest("user profile is managed by an external provider")
			}
		}
		if input.FirstName != nil {
			trimmed := strings.TrimSpace(*input.FirstName)
			if trimmed != user.FirstName {
				return nil, apperrors.NewBadRequest("user profile is managed by an external provider")
			}
		}
		if input.LastName != nil {
			trimmed := strings.TrimSpace(*input.LastName)
			if trimmed != user.LastName {
				return nil, apperrors.NewBadRequest("user profile is managed by an external provider")
			}
		}
		if input.Avatar != nil {
			trimmed := strings.TrimSpace(*input.Avatar)
			if trimmed != strings.TrimSpace(user.Avatar) {
				return nil, apperrors.NewBadRequest("user profile is managed by an external provider")
			}
		}
	}

	updates := map[string]any{}

	if input.Username != nil {
		if name := strings.TrimSpace(*input.Username); name != "" && name != user.Username {
			updates["username"] = name
		}
	}
	if input.Email != nil {
		if email := strings.ToLower(strings.TrimSpace(*input.Email)); email != "" && email != user.Email {
			updates["email"] = email
		}
	}
	if input.FirstName != nil {
		updates["first_name"] = strings.TrimSpace(*input.FirstName)
	}
	if input.LastName != nil {
		updates["last_name"] = strings.TrimSpace(*input.LastName)
	}
	if input.Avatar != nil {
		updates["avatar"] = strings.TrimSpace(*input.Avatar)
	}

	if len(updates) == 0 {
		return &user, nil
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return nil, apperrors.NewBadRequest("username or email already exists")
		}
		return nil, fmt.Errorf("user service: update user: %w", err)
	}

	if err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("user service: reload user: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "user.update",
		Resource: user.ID,
		Result:   "success",
		Metadata: updates,
	})

	return &user, nil
}

// SetRoles replaces role assignments for the specified user.
func (s *UserService) SetRoles(ctx context.Context, id string, roleIDs []string) (*models.User, error) {
	ctx = ensureContext(ctx)

	userID := strings.TrimSpace(id)
	if userID == "" {
		return nil, apperrors.NewBadRequest("user id is required")
	}

	cleanIDs := normaliseIDs(roleIDs)

	var result *models.User

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Preload("Roles").First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("user service: load user: %w", err)
		}

		var roles []models.Role
		if len(cleanIDs) > 0 {
			if err := tx.Where("id IN ?", cleanIDs).Find(&roles).Error; err != nil {
				return fmt.Errorf("user service: load roles: %w", err)
			}
			if len(roles) != len(cleanIDs) {
				return apperrors.NewBadRequest("one or more roles were not found")
			}
		}

		if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
			return fmt.Errorf("user service: replace roles: %w", err)
		}

		if err := tx.
			Preload("Roles").
			Preload("Teams").
			Preload("Teams.Roles").
			First(&user, "id = ?", userID).Error; err != nil {
			return fmt.Errorf("user service: reload user: %w", err)
		}

		result = &user

		recordAudit(s.auditService, ctx, AuditEntry{
			Action:   "user.set_roles",
			Resource: user.ID,
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

// Delete removes a user unless the account is marked as root.
func (s *UserService) Delete(ctx context.Context, id string) error {
	ctx = ensureContext(ctx)

	var user models.User
	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("user service: load user: %w", err)
	}

	if user.IsRoot {
		return ErrRootUserImmutable
	}

	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Clear(); err != nil {
		return fmt.Errorf("user service: clear user roles: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Teams").Clear(); err != nil {
		return fmt.Errorf("user service: clear user teams: %w", err)
	}

	if err := s.db.WithContext(ctx).Delete(&user).Error; err != nil {
		return fmt.Errorf("user service: delete user: %w", err)
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "user.delete",
		Resource: user.ID,
		Result:   "success",
	})

	return nil
}

// SetActive toggles the active state of an account.
func (s *UserService) SetActive(ctx context.Context, id string, active bool) error {
	ctx = ensureContext(ctx)

	var user models.User
	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("user service: load user: %w", err)
	}

	if user.IsRoot && !active {
		return ErrRootUserImmutable
	}

	if err := s.db.WithContext(ctx).Model(&user).Update("is_active", active).Error; err != nil {
		return fmt.Errorf("user service: update active state: %w", err)
	}

	action := "user.activate"
	if !active {
		action = "user.deactivate"
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   action,
		Resource: user.ID,
		Result:   "success",
	})

	return nil
}

// ChangePassword hashes and updates the user's password.
func (s *UserService) ChangePassword(ctx context.Context, id, newPassword string) error {
	ctx = ensureContext(ctx)

	if strings.TrimSpace(newPassword) == "" {
		return apperrors.NewBadRequest("new password is required")
	}

	hashed, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("user service: hash new password: %w", err)
	}

	result := s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Update("password", hashed)

	if result.Error != nil {
		return fmt.Errorf("user service: change password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}

	recordAudit(s.auditService, ctx, AuditEntry{
		Action:   "user.password_change",
		Resource: id,
		Result:   "success",
	})

	return nil
}
