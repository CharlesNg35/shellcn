package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	testutil "github.com/charlesng35/shellcn/internal/testutil"
)

func TestSSOResolveExistingUser(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	clock := &testClock{current: time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)}
	jwtService, err := NewJWTService(JWTConfig{
		Secret:         "sso-secret",
		AccessTokenTTL: time.Hour,
		Clock:          clock.Now,
	})
	require.NoError(t, err)

	sessionService, err := NewSessionService(db, jwtService, SessionConfig{
		RefreshTokenTTL: time.Hour,
		RefreshLength:   32,
		Clock:           clock.Now,
	})
	require.NoError(t, err)

	manager, err := NewSSOManager(db, sessionService, SSOConfig{Clock: clock.Now})
	require.NoError(t, err)

	user := createTestUser(t, db, "existing")

	identity := providers.Identity{
		Provider: "oidc",
		Subject:  "user-123",
		Email:    user.Email,
		RawClaims: map[string]any{
			"department": "engineering",
		},
		Groups: []string{"admins"},
	}

	tokens, resolvedUser, session, err := manager.Resolve(context.Background(), identity, ResolveOptions{
		AutoProvision: false,
		SessionMeta: SessionMetadata{
			IPAddress: "10.1.1.1",
			UserAgent: "browser",
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	require.NotNil(t, session)
	require.Equal(t, user.ID, resolvedUser.ID)

	var stored models.User
	require.NoError(t, db.Take(&stored, "id = ?", user.ID).Error)
	require.NotNil(t, stored.LastLoginAt)
	require.WithinDuration(t, clock.Now(), stored.LastLoginAt.UTC(), time.Second)
	require.Equal(t, "10.1.1.1", stored.LastLoginIP)
}

func TestSSOResolveAutoProvision(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	clock := &testClock{current: time.Date(2024, 3, 1, 9, 30, 0, 0, time.UTC)}
	jwtService, err := NewJWTService(JWTConfig{
		Secret:         "sso-secret",
		AccessTokenTTL: time.Hour,
		Clock:          clock.Now,
	})
	require.NoError(t, err)

	sessionService, err := NewSessionService(db, jwtService, SessionConfig{
		RefreshTokenTTL: time.Hour,
		RefreshLength:   32,
		Clock:           clock.Now,
	})
	require.NoError(t, err)

	manager, err := NewSSOManager(db, sessionService, SSOConfig{Clock: clock.Now})
	require.NoError(t, err)

	identity := providers.Identity{
		Provider:  "saml",
		Subject:   "abc-123",
		Email:     "New.User+SAML@example.com",
		FirstName: "New",
		LastName:  "User",
		Groups:    []string{"buyers"},
	}

	tokens, resolvedUser, session, err := manager.Resolve(context.Background(), identity, ResolveOptions{
		AutoProvision: true,
		SessionMeta: SessionMetadata{
			IPAddress: "203.0.113.42 ",
			UserAgent: "saml-client",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)

	require.Equal(t, "new-user-saml", resolvedUser.Username)
	require.Equal(t, "new.user+saml@example.com", resolvedUser.Email)
	require.True(t, resolvedUser.IsActive)

	var stored models.User
	require.NoError(t, db.Take(&stored, "id = ?", resolvedUser.ID).Error)
	require.NotNil(t, stored.LastLoginAt)
	require.Equal(t, "203.0.113.42", stored.LastLoginIP)

	var count int64
	require.NoError(t, db.Model(&models.User{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSSOResolveFailsWithoutEmail(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	clock := &testClock{current: time.Now().UTC()}
	jwtService, err := NewJWTService(JWTConfig{
		Secret:         "sso-secret",
		AccessTokenTTL: time.Hour,
		Clock:          func() time.Time { return clock.Now() },
	})
	require.NoError(t, err)

	sessionService, err := NewSessionService(db, jwtService, SessionConfig{
		RefreshTokenTTL: time.Hour,
		RefreshLength:   32,
		Clock:           func() time.Time { return clock.Now() },
	})
	require.NoError(t, err)

	manager, err := NewSSOManager(db, sessionService, SSOConfig{Clock: func() time.Time { return clock.Now() }})
	require.NoError(t, err)

	identity := providers.Identity{
		Provider: "ldap",
		Subject:  "no-email",
	}

	_, _, _, err = manager.Resolve(context.Background(), identity, ResolveOptions{})
	require.ErrorIs(t, err, ErrSSOEmailRequired)
}
