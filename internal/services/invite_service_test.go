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

func TestInviteServiceGenerateAndRedeem(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(24*time.Hour),
	)
	require.NoError(t, err)

	token, link, err := svc.GenerateInvite(context.Background(), "user@example.com", "admin")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, link)

	var invite models.UserInvite
	require.NoError(t, db.First(&invite).Error)
	require.Equal(t, "user@example.com", invite.Email)
	require.Nil(t, invite.AcceptedAt)

	accepted, err := svc.RedeemInvite(context.Background(), token)
	require.NoError(t, err)
	require.NotNil(t, accepted.AcceptedAt)

	// Redeeming again should fail with already used.
	_, err = svc.RedeemInvite(context.Background(), token)
	require.ErrorIs(t, err, ErrInviteAlreadyUsed)
}

func TestInviteServiceExpiry(t *testing.T) {
	db := openInviteTestDB(t)
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc, err := NewInviteService(db, nil,
		WithInviteClock(func() time.Time { return current }),
		WithInviteExpiry(time.Hour),
	)
	require.NoError(t, err)

	token, _, err := svc.GenerateInvite(context.Background(), "late@example.com", "admin")
	require.NoError(t, err)

	current = current.Add(2 * time.Hour)

	_, err = svc.RedeemInvite(context.Background(), token)
	require.ErrorIs(t, err, ErrInviteExpired)
}

func openInviteTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&models.UserInvite{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}
