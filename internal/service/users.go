package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

// ErrWrongPassword is returned when a self password change supplies the wrong
// current password.
var ErrWrongPassword = errors.New("service: current password is incorrect")

// UserService manages platform accounts: it hashes passwords on write and never
// returns hashes (the store clears them on read).
type UserService struct {
	users store.UserStore
}

func NewUserService(users store.UserStore) *UserService {
	return &UserService{users: users}
}

// NewUserInput describes an account to create (password in plaintext).
type NewUserInput struct {
	Username    string
	Email       string
	DisplayName string
	Roles       []models.Role
	Password    string
}

func (s *UserService) Create(ctx context.Context, in NewUserInput) (models.User, error) {
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return models.User{}, err
	}
	now := time.Now()
	user := models.User{
		ID:          uuid.NewString(),
		Username:    in.Username,
		Email:       in.Email,
		DisplayName: in.DisplayName,
		Roles:       in.Roles,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.users.Create(ctx, &user, hash); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.users.List(ctx)
}

func (s *UserService) Get(ctx context.Context, id string) (models.User, error) {
	return s.users.GetByID(ctx, id)
}

// UpdateUserInput changes an account's profile, roles, and enabled state.
type UpdateUserInput struct {
	Email       string
	DisplayName string
	Roles       []models.Role
	Disabled    bool
}

func (s *UserService) Update(ctx context.Context, id string, in UpdateUserInput) (models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return models.User{}, err
	}
	user.Email = in.Email
	user.DisplayName = in.DisplayName
	user.Roles = in.Roles
	user.Disabled = in.Disabled
	user.UpdatedAt = time.Now()
	if err := s.users.Update(ctx, &user); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.users.Delete(ctx, id)
}

// UpdateProfile changes only a user's own profile fields (display name + email).
// Username, roles, and enabled state are intentionally left untouched.
func (s *UserService) UpdateProfile(ctx context.Context, id, email, displayName string) (models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return models.User{}, err
	}
	user.Email = email
	user.DisplayName = displayName
	user.UpdatedAt = time.Now()
	if err := s.users.Update(ctx, &user); err != nil {
		return models.User{}, err
	}
	return user, nil
}

// ChangePassword verifies the current password before setting a new one.
func (s *UserService) ChangePassword(ctx context.Context, id, current, next string) error {
	hash, err := s.users.GetPasswordHash(ctx, id)
	if err != nil {
		return err
	}
	ok, err := auth.VerifyPassword(hash, current)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWrongPassword
	}
	newHash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	return s.users.SetPasswordHash(ctx, id, newHash)
}
