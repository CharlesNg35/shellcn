package providers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

var (
	// ErrInvalidCredentials is returned when the supplied identity/password pair is invalid.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	// ErrAccountLocked signals that the user has exceeded the permitted failed attempts.
	ErrAccountLocked = errors.New("auth: account locked")
	// ErrAccountDisabled signals that the user has been deactivated.
	ErrAccountDisabled = errors.New("auth: account disabled")
)

// LocalConfig defines tunable behaviour for the local provider.
type LocalConfig struct {
	LockoutThreshold int
	LockoutDuration  time.Duration
	Clock            func() time.Time
}

// AuthenticateInput contains metadata required to authenticate a local user.
type AuthenticateInput struct {
	Identifier string
	Password   string
	IPAddress  string
	UserAgent  string
}

// RegisterInput captures the details required to register a new local user.
type RegisterInput struct {
	Username  string
	Email     string
	Password  string
	FirstName string
	LastName  string
}

// LocalProvider implements username/password authentication with account lockout controls.
type LocalProvider struct {
	db        *gorm.DB
	clock     func() time.Time
	threshold int
	duration  time.Duration
}

// NewLocalProvider builds a provider with sane defaults.
func NewLocalProvider(db *gorm.DB, cfg LocalConfig) (*LocalProvider, error) {
	if db == nil {
		return nil, errors.New("local provider: db is required")
	}

	threshold := cfg.LockoutThreshold
	if threshold <= 0 {
		threshold = 5
	}

	duration := cfg.LockoutDuration
	if duration <= 0 {
		duration = 15 * time.Minute
	}

	clock := time.Now
	if cfg.Clock != nil {
		clock = cfg.Clock
	}

	return &LocalProvider{
		db:        db,
		clock:     clock,
		threshold: threshold,
		duration:  duration,
	}, nil
}

// Authenticate verifies the supplied credentials and returns the associated user when successful.
func (p *LocalProvider) Authenticate(input AuthenticateInput) (*models.User, error) {
	identity := strings.TrimSpace(input.Identifier)
	if identity == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	var user models.User
	err := p.db.Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", identity, identity).
		Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("local provider: query user: %w", err)
	}

	now := p.clock()

	if !user.IsActive {
		return nil, ErrAccountDisabled
	}

	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return nil, ErrAccountLocked
	}

	// Unlock the account if the lockout duration has elapsed.
	if user.LockedUntil != nil && !user.LockedUntil.After(now) {
		user.LockedUntil = nil
		user.FailedAttempts = 0
		if err := p.db.Model(&user).Updates(map[string]any{
			"locked_until":    nil,
			"failed_attempts": 0,
		}).Error; err != nil {
			return nil, fmt.Errorf("local provider: reset lock state: %w", err)
		}
	}

	if !crypto.VerifyPassword(user.Password, input.Password) {
		return nil, p.handleFailedAttempt(&user, now)
	}

	user.FailedAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = &now
	user.LastLoginIP = strings.TrimSpace(input.IPAddress)

	if err := p.db.Model(&user).Updates(map[string]any{
		"failed_attempts": 0,
		"locked_until":    nil,
		"last_login_at":   now,
		"last_login_ip":   user.LastLoginIP,
	}).Error; err != nil {
		return nil, fmt.Errorf("local provider: update user: %w", err)
	}

	return &user, nil
}

func (p *LocalProvider) handleFailedAttempt(user *models.User, now time.Time) error {
	user.FailedAttempts++

	updates := map[string]any{
		"failed_attempts": user.FailedAttempts,
	}

	if user.FailedAttempts >= p.threshold {
		lockUntil := now.Add(p.duration)
		user.LockedUntil = &lockUntil
		updates["locked_until"] = lockUntil
	}

	if err := p.db.Model(user).Updates(updates).Error; err != nil {
		return fmt.Errorf("local provider: update failed attempts: %w", err)
	}

	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return ErrAccountLocked
	}

	return ErrInvalidCredentials
}

// Register creates a new local user with a hashed password.
func (p *LocalProvider) Register(input RegisterInput) (*models.User, error) {
	if strings.TrimSpace(input.Username) == "" || strings.TrimSpace(input.Email) == "" || input.Password == "" {
		return nil, errors.New("local provider: username, email and password are required")
	}

	hashed, err := crypto.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("local provider: hash password: %w", err)
	}

	user := &models.User{
		Username:  strings.TrimSpace(input.Username),
		Email:     strings.ToLower(strings.TrimSpace(input.Email)),
		Password:  hashed,
		FirstName: strings.TrimSpace(input.FirstName),
		LastName:  strings.TrimSpace(input.LastName),
		IsActive:  true,
	}

	if err := p.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("local provider: create user: %w", err)
	}

	return user, nil
}

// ChangePassword updates a user's password after verifying the existing credential.
func (p *LocalProvider) ChangePassword(userID, currentPassword, newPassword string) error {
	if strings.TrimSpace(userID) == "" || newPassword == "" {
		return errors.New("local provider: user id and new password are required")
	}

	var user models.User
	if err := p.db.Take(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("local provider: find user: %w", err)
	}

	if currentPassword != "" && !crypto.VerifyPassword(user.Password, currentPassword) {
		return ErrInvalidCredentials
	}

	hashed, err := crypto.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("local provider: hash password: %w", err)
	}

	if err := p.db.Model(&user).Update("password", hashed).Error; err != nil {
		return fmt.Errorf("local provider: update password: %w", err)
	}

	return nil
}
