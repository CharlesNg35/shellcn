package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestEmailVerificationCreateAndVerify(t *testing.T) {
	db := openVerificationTestDB(t)
	current := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)

	svc, err := NewEmailVerificationService(db, nil,
		WithVerificationClock(func() time.Time { return current }),
		WithVerificationExpiry(12*time.Hour),
	)
	require.NoError(t, err)

	token, link, err := svc.CreateToken(context.Background(), "user-123", "user@example.com")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)

	var stored models.EmailVerification
	require.NoError(t, db.First(&stored).Error)
	require.Equal(t, "user-123", stored.UserID)
	require.Nil(t, stored.VerifiedAt)

	verified, err := svc.VerifyToken(context.Background(), token)
	require.NoError(t, err)
	require.NotNil(t, verified.VerifiedAt)

	// Second verification attempt should fail.
	_, err = svc.VerifyToken(context.Background(), token)
	require.ErrorIs(t, err, ErrVerificationUsed)
}

func TestEmailVerificationExpiry(t *testing.T) {
	db := openVerificationTestDB(t)
	current := time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC)

	svc, err := NewEmailVerificationService(db, nil,
		WithVerificationClock(func() time.Time { return current }),
		WithVerificationExpiry(time.Hour),
	)
	require.NoError(t, err)

	token, _, err := svc.CreateToken(context.Background(), "user-789", "verify@example.com")
	require.NoError(t, err)

	current = current.Add(2 * time.Hour)

	_, err = svc.VerifyToken(context.Background(), token)
	require.ErrorIs(t, err, ErrVerificationExpired)
}

func openVerificationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.EmailVerification{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
