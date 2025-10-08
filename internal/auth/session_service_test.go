package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/models"
	testutil "github.com/charlesng35/shellcn/internal/testutil"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestCreateSessionGeneratesTokens(t *testing.T) {
	db, svc, clock := setupSessionService(t)

	user := createTestUser(t, db, "user-create")

	tokens, session, err := svc.CreateSession(user.ID, SessionMetadata{
		IPAddress: "10.0.0.1 ",
		UserAgent: "unit-test",
		Device:    "laptop",
		Claims:    map[string]any{"role": "admin"},
	})
	require.NoError(t, err)

	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	require.NotNil(t, session)
	require.Equal(t, user.ID, session.UserID)
	require.Equal(t, "10.0.0.1", session.IPAddress)
	require.Equal(t, "unit-test", session.UserAgent)
	require.Equal(t, "laptop", session.DeviceName)

	var reloaded models.Session
	require.NoError(t, db.Take(&reloaded, "id = ?", session.ID).Error)
	require.Equal(t, tokens.RefreshToken, reloaded.RefreshToken)
	require.True(t, reloaded.ExpiresAt.After(clock.Now()))
	require.True(t, reloaded.LastUsedAt.Equal(clock.Now()))
}

func TestRefreshSessionRotatesToken(t *testing.T) {
	db, svc, clock := setupSessionService(t)
	user := createTestUser(t, db, "user-refresh")

	tokens, session, err := svc.CreateSession(user.ID, SessionMetadata{})
	require.NoError(t, err)

	clock.Advance(5 * time.Minute)

	newTokens, updatedSession, err := svc.RefreshSession(tokens.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, tokens.RefreshToken, newTokens.RefreshToken)
	require.NotEqual(t, tokens.AccessToken, newTokens.AccessToken)

	require.Equal(t, session.ID, updatedSession.ID)
	require.Equal(t, newTokens.RefreshToken, updatedSession.RefreshToken)
	require.True(t, updatedSession.LastUsedAt.Equal(clock.Now()))

	_, _, err = svc.RefreshSession(tokens.RefreshToken)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestRefreshSessionExpired(t *testing.T) {
	db, svc, clock := setupSessionService(t)
	user := createTestUser(t, db, "user-expired")

	tokens, session, err := svc.CreateSession(user.ID, SessionMetadata{})
	require.NoError(t, err)

	require.NoError(t, db.Model(&models.Session{}).
		Where("id = ?", session.ID).
		Update("expires_at", clock.Now().Add(-time.Minute)).Error)

	_, _, err = svc.RefreshSession(tokens.RefreshToken)
	require.ErrorIs(t, err, ErrSessionExpired)
}

func TestRevokeSessionPreventsRefresh(t *testing.T) {
	db, svc, clock := setupSessionService(t)

	user := createTestUser(t, db, "user-revoke")

	tokens, session, err := svc.CreateSession(user.ID, SessionMetadata{})
	require.NoError(t, err)

	require.NoError(t, svc.RevokeSession(session.ID))

	err = svc.RevokeSession("non-existent")
	require.ErrorIs(t, err, ErrSessionNotFound)

	_, _, err = svc.RefreshSession(tokens.RefreshToken)
	require.ErrorIs(t, err, ErrSessionRevoked)

	var stored models.Session
	require.NoError(t, db.Take(&stored, "id = ?", session.ID).Error)
	require.NotNil(t, stored.RevokedAt)
	require.True(t, stored.RevokedAt.After(clock.Now().Add(-time.Nanosecond)))
}

func setupSessionService(t *testing.T) (*gorm.DB, *SessionService, *testClock) {
	t.Helper()

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	clock := &testClock{current: time.Date(2024, 1, 3, 9, 0, 0, 0, time.UTC)}

	jwtService, err := NewJWTService(JWTConfig{
		Secret:         "session-secret",
		AccessTokenTTL: time.Hour,
		Clock:          clock.Now,
	})
	require.NoError(t, err)

	sessionService, err := NewSessionService(db, jwtService, SessionConfig{
		RefreshTokenTTL: 2 * time.Hour,
		RefreshLength:   24,
		Clock:           clock.Now,
	})
	require.NoError(t, err)

	return db, sessionService, clock
}

func createTestUser(t *testing.T, db *gorm.DB, username string) *models.User {
	t.Helper()

	hashed, err := crypto.HashPassword("password")
	require.NoError(t, err)

	user := &models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: hashed,
		IsActive: true,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Model(user).Update("is_active", true).Error)
	return user
}

type testClock struct {
	current time.Time
}

func (c *testClock) Now() time.Time {
	return c.current
}

func (c *testClock) Advance(d time.Duration) {
	c.current = c.current.Add(d)
}
