package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestNewJWTServiceRequiresSecret(t *testing.T) {
	_, err := NewJWTService(JWTConfig{})
	require.Error(t, err)
	require.EqualError(t, err, "jwt: secret must be provided")
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	current := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }

	svc, err := NewJWTService(JWTConfig{
		Secret:         "super-secret",
		Issuer:         "shellcn",
		AccessTokenTTL: time.Hour,
		Clock:          now,
	})
	require.NoError(t, err)

	inputMeta := map[string]any{"role": "admin"}
	token, err := svc.GenerateAccessToken(AccessTokenInput{
		UserID:    "user-123",
		SessionID: "session-456",
		Audience:  []string{"api"},
		Metadata:  inputMeta,
	})
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Ensure metadata cloning protects from external mutation.
	inputMeta["role"] = "user"

	claims, err := svc.ValidateAccessToken(token)
	require.NoError(t, err)

	require.Equal(t, "user-123", claims.UserID)
	require.Equal(t, "session-456", claims.SessionID)
	require.Equal(t, "shellcn", claims.Issuer)
	require.Equal(t, jwt.ClaimStrings{"api"}, claims.Audience)
	require.Equal(t, "admin", claims.Metadata["role"])
	require.True(t, claims.IssuedAt.Time.Equal(current))
	require.True(t, claims.ExpiresAt.Time.Equal(current.Add(time.Hour)))
}

func TestValidateAccessTokenInvalidSignature(t *testing.T) {
	now := func() time.Time { return time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC) }

	issuer, err := NewJWTService(JWTConfig{
		Secret:         "issuer-secret",
		AccessTokenTTL: time.Minute,
		Clock:          now,
	})
	require.NoError(t, err)

	token, err := issuer.GenerateAccessToken(AccessTokenInput{UserID: "user-123"})
	require.NoError(t, err)

	verifier, err := NewJWTService(JWTConfig{
		Secret:         "other-secret",
		AccessTokenTTL: time.Minute,
		Clock:          now,
	})
	require.NoError(t, err)

	_, err = verifier.ValidateAccessToken(token)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwt.ErrTokenSignatureInvalid))
}

func TestValidateAccessTokenExpired(t *testing.T) {
	current := time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)
	now := func() time.Time { return current }

	svc, err := NewJWTService(JWTConfig{
		Secret:         "secret",
		AccessTokenTTL: time.Minute,
		Clock:          now,
	})
	require.NoError(t, err)

	token, err := svc.GenerateAccessToken(AccessTokenInput{UserID: "user-123"})
	require.NoError(t, err)

	// Move time forward beyond expiry.
	current = current.Add(2 * time.Minute)

	_, err = svc.ValidateAccessToken(token)
	require.Error(t, err)
	require.True(t, errors.Is(err, jwt.ErrTokenExpired))
}
