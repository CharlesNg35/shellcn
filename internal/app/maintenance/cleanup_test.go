package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	testutil "github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestCleanupTokens(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	now := time.Date(2024, 2, 10, 15, 0, 0, 0, time.UTC)

	expiredReset := models.PasswordResetToken{
		UserID:    "user-expired",
		Token:     "expired",
		ExpiresAt: now.Add(-time.Hour),
	}
	activeReset := models.PasswordResetToken{
		UserID:    "user-active",
		Token:     "active",
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, db.Create(&expiredReset).Error)
	require.NoError(t, db.Create(&activeReset).Error)

	expiredInvite := models.UserInvite{
		Email:     "expired@example.com",
		TokenHash: "invite-expired",
		ExpiresAt: now.Add(-time.Hour),
	}
	activeInvite := models.UserInvite{
		Email:     "active@example.com",
		TokenHash: "invite-active",
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, db.Create(&expiredInvite).Error)
	require.NoError(t, db.Create(&activeInvite).Error)

	expiredVerification := models.EmailVerification{
		UserID:    "user-expired",
		TokenHash: "verify-expired",
		ExpiresAt: now.Add(-time.Hour),
	}
	activeVerification := models.EmailVerification{
		UserID:    "user-active",
		TokenHash: "verify-active",
		ExpiresAt: now.Add(time.Hour),
	}
	require.NoError(t, db.Create(&expiredVerification).Error)
	require.NoError(t, db.Create(&activeVerification).Error)

	stats, err := CleanupTokens(context.Background(), db, now)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.PasswordResets)
	require.Equal(t, int64(1), stats.Invites)
	require.Equal(t, int64(1), stats.EmailVerifications)

	assertRemaining := func(model any, expected int64) {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		require.Equal(t, expected, count)
	}

	assertRemaining(&models.PasswordResetToken{}, 1)
	assertRemaining(&models.UserInvite{}, 1)
	assertRemaining(&models.EmailVerification{}, 1)
}

func TestCleanerRunOnce(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         "cleanup-secret",
		Issuer:         "test-suite",
		AccessTokenTTL: time.Hour,
	})
	require.NoError(t, err)

	clock := fixedClock{current: time.Date(2024, 5, 20, 9, 0, 0, 0, time.UTC)}

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, iauth.SessionConfig{
		RefreshTokenTTL: time.Hour,
		RefreshLength:   16,
		Clock:           clock.Now,
	})
	require.NoError(t, err)

	user := seedUser(t, db, "cleanup-user")

	_, expiredSession, err := sessionSvc.CreateSession(user.ID, iauth.SessionMetadata{})
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.Session{}).Where("id = ?", expiredSession.ID).
		Update("expires_at", clock.Now().Add(-2*time.Hour)).Error)

	_, activeSession, err := sessionSvc.CreateSession(user.ID, iauth.SessionMetadata{})
	require.NoError(t, err)

	_, revokedSession, err := sessionSvc.CreateSession(user.ID, iauth.SessionMetadata{})
	require.NoError(t, err)
	require.NoError(t, sessionSvc.RevokeSession(revokedSession.ID))

	// Seed audit log older than retention window.
	require.NoError(t, auditSvc.Log(context.Background(), services.AuditEntry{
		Action:   "test.action",
		Result:   "success",
		Username: "tester",
	}))
	var auditLog models.AuditLog
	require.NoError(t, db.First(&auditLog).Error)
	oldTimestamp := clock.Now().AddDate(0, 0, -10)
	require.NoError(t, db.Model(&auditLog).Update("created_at", oldTimestamp).Error)

	// Seed tokens
	require.NoError(t, db.Create(&models.PasswordResetToken{
		UserID:    user.ID,
		Token:     "reset-expired",
		ExpiresAt: clock.Now().Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&models.UserInvite{
		Email:     "invite@example.com",
		TokenHash: "invite-hash",
		ExpiresAt: clock.Now().Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&models.EmailVerification{
		UserID:    user.ID,
		TokenHash: "verify-hash",
		ExpiresAt: clock.Now().Add(-time.Hour),
	}).Error)

	c := NewCleaner(db, sessionSvc, auditSvc,
		WithNow(clock.Now),
		WithAuditRetentionDays(7),
		WithCron(cron.New(cron.WithLogger(cron.DiscardLogger))),
	)

	require.NoError(t, c.RunOnce(context.Background()))

	assertNotFound := func(id string) {
		var s models.Session
		err := db.First(&s, "id = ?", id).Error
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}

	assertNotFound(expiredSession.ID)
	assertNotFound(revokedSession.ID)

	var remaining models.Session
	require.NoError(t, db.First(&remaining, "id = ?", activeSession.ID).Error)

	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).Count(&auditCount).Error)
	require.Equal(t, int64(0), auditCount)

	var tokenCount int64
	require.NoError(t, db.Model(&models.PasswordResetToken{}).Count(&tokenCount).Error)
	require.Equal(t, int64(0), tokenCount)
	require.NoError(t, db.Model(&models.UserInvite{}).Count(&tokenCount).Error)
	require.Equal(t, int64(0), tokenCount)
	require.NoError(t, db.Model(&models.EmailVerification{}).Count(&tokenCount).Error)
	require.Equal(t, int64(0), tokenCount)
}

func seedUser(t *testing.T, db *gorm.DB, username string) *models.User {
	t.Helper()

	hash, err := crypto.HashPassword("Password123!")
	require.NoError(t, err)

	user := &models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: hash,
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

type fixedClock struct {
	current time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.current
}
