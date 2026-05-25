package auth

import (
	"context"
	"errors"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

var (
	// ErrInvalidCredentials is returned for a bad username/password pair.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	// ErrAccountDisabled is returned when the account exists but is disabled.
	ErrAccountDisabled = errors.New("auth: account disabled")
	// ErrNotImplemented is returned by stubbed authenticators (e.g. OIDC in v1).
	ErrNotImplemented = errors.New("auth: not implemented")
)

// Authenticator verifies a principal and returns the authenticated user.
// Local accounts ship in v1; OIDC implements the same interface later.
type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (models.User, error)
}

// LocalAuthenticator verifies username/password against the user store.
type LocalAuthenticator struct {
	users store.UserStore
}

// NewLocalAuthenticator wires the user store.
func NewLocalAuthenticator(users store.UserStore) *LocalAuthenticator {
	return &LocalAuthenticator{users: users}
}

// Authenticate looks up the user, verifies the password, and rejects disabled
// accounts. It returns ErrInvalidCredentials for both unknown users and wrong
// passwords (no user-enumeration signal).
func (a *LocalAuthenticator) Authenticate(ctx context.Context, username, password string) (models.User, error) {
	user, err := a.users.GetByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, err
	}
	hash, err := a.users.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return models.User{}, err
	}
	ok, err := VerifyPassword(hash, password)
	if err != nil {
		return models.User{}, err
	}
	if !ok {
		return models.User{}, ErrInvalidCredentials
	}
	if user.Disabled {
		return models.User{}, ErrAccountDisabled
	}
	return user, nil
}

// OIDCAuthenticator is the day-one interface placeholder; v1 ships local only.
type OIDCAuthenticator struct{}

// Authenticate always reports not-implemented in v1.
func (OIDCAuthenticator) Authenticate(context.Context, string, string) (models.User, error) {
	return models.User{}, ErrNotImplemented
}

// MFAVerifier is the optional second-factor hook (TOTP). Interface only in v1.
type MFAVerifier interface {
	Verify(ctx context.Context, userID, code string) (bool, error)
}
