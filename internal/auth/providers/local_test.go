package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestAuthenticateSuccessResetsCounters(t *testing.T) {
	db := setupDB(t)
	current := time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }

	provider := newLocalProvider(t, db, LocalConfig{Clock: now})

	hashed, err := crypto.HashPassword("password123")
	require.NoError(t, err)

	user := models.User{
		Username:       "alice",
		Email:          "alice@example.com",
		Password:       hashed,
		IsActive:       true,
		FailedAttempts: 3,
	}
	require.NoError(t, db.Create(&user).Error)

	result, err := provider.Authenticate(AuthenticateInput{
		Identifier: "alice",
		Password:   "password123",
		IPAddress:  "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, user.ID, result.ID)

	var updated models.User
	require.NoError(t, db.Take(&updated, "id = ?", user.ID).Error)

	require.Equal(t, 0, updated.FailedAttempts)
	require.Nil(t, updated.LockedUntil)
	require.NotNil(t, updated.LastLoginAt)
	require.True(t, updated.LastLoginAt.Equal(current))
	require.Equal(t, "127.0.0.1", updated.LastLoginIP)
}

func TestAuthenticateInvalidPasswordLocksAccount(t *testing.T) {
	db := setupDB(t)
	current := time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }

	provider := newLocalProvider(t, db, LocalConfig{
		LockoutThreshold: 3,
		LockoutDuration:  10 * time.Minute,
		Clock:            now,
	})

	hashed, err := crypto.HashPassword("correct")
	require.NoError(t, err)

	user := models.User{
		Username:       "bob",
		Email:          "bob@example.com",
		Password:       hashed,
		IsActive:       true,
		FailedAttempts: 2,
	}
	require.NoError(t, db.Create(&user).Error)

	err = tryAuthenticate(provider, "bob", "wrong")
	require.ErrorIs(t, err, ErrAccountLocked)

	var updated models.User
	require.NoError(t, db.Take(&updated, "id = ?", user.ID).Error)

	require.Equal(t, 3, updated.FailedAttempts)
	require.NotNil(t, updated.LockedUntil)
	require.WithinDuration(t, current.Add(10*time.Minute), *updated.LockedUntil, time.Second)
}

func TestAuthenticateLockedAccount(t *testing.T) {
	db := setupDB(t)
	current := time.Date(2024, 1, 2, 11, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }

	provider := newLocalProvider(t, db, LocalConfig{Clock: now})

	hashed, err := crypto.HashPassword("correct")
	require.NoError(t, err)

	lockUntil := current.Add(5 * time.Minute)
	user := models.User{
		Username:       "charlie",
		Email:          "charlie@example.com",
		Password:       hashed,
		IsActive:       true,
		LockedUntil:    &lockUntil,
		LastLoginIP:    "",
		LastLoginAt:    nil,
		FailedAttempts: 5,
	}
	require.NoError(t, db.Create(&user).Error)

	err = tryAuthenticate(provider, "charlie@example.com", "correct")
	require.ErrorIs(t, err, ErrAccountLocked)
}

func TestAuthenticateDisabledAccount(t *testing.T) {
	db := setupDB(t)

	provider := newLocalProvider(t, db, LocalConfig{})

	hashed, err := crypto.HashPassword("correct")
	require.NoError(t, err)

	user := models.User{
		Username: "diana",
		Email:    "diana@example.com",
		Password: hashed,
		IsActive: false,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&user).Update("is_active", false).Error)

	err = tryAuthenticate(provider, "diana", "correct")
	require.ErrorIs(t, err, ErrAccountDisabled)
}

func TestRegisterHashesPassword(t *testing.T) {
	db := setupDB(t)
	provider := newLocalProvider(t, db, LocalConfig{})

	user, err := provider.Register(RegisterInput{
		Username: "eve",
		Email:    "eve@example.com",
		Password: "secret",
	})
	require.NoError(t, err)

	require.NotEqual(t, "secret", user.Password)
	require.True(t, crypto.VerifyPassword(user.Password, "secret"))
}

func TestChangePassword(t *testing.T) {
	db := setupDB(t)
	provider := newLocalProvider(t, db, LocalConfig{})

	user, err := provider.Register(RegisterInput{
		Username: "frank",
		Email:    "frank@example.com",
		Password: "initial",
	})
	require.NoError(t, err)

	require.NoError(t, provider.ChangePassword(user.ID, "initial", "updated"))

	var updated models.User
	require.NoError(t, db.Take(&updated, "id = ?", user.ID).Error)
	require.True(t, crypto.VerifyPassword(updated.Password, "updated"))

	err = provider.ChangePassword(user.ID, "wrong", "another")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func tryAuthenticate(provider *LocalProvider, identifier, password string) error {
	_, err := provider.Authenticate(AuthenticateInput{
		Identifier: identifier,
		Password:   password,
	})
	return err
}

func newLocalProvider(t *testing.T, db *gorm.DB, cfg LocalConfig) *LocalProvider {
	t.Helper()
	provider, err := NewLocalProvider(db, cfg)
	require.NoError(t, err)
	return provider
}

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := database.Open(database.Config{Driver: "sqlite"})
	require.NoError(t, err)

	require.NoError(t, database.AutoMigrate(db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
