package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStateCodecRoundTrip(t *testing.T) {
	codec, err := NewStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Minute, nil)
	require.NoError(t, err)

	token, err := codec.Encode(StatePayload{
		Provider:  "oidc",
		ReturnURL: "/dashboard",
		ErrorURL:  "/login?error=sso",
		Nonce:     "nonce",
		PKCE:      "verifier",
		RequestID: "request-123",
	})
	require.NoError(t, err)
	require.NotEmpty(t, token)

	payload, err := codec.Decode(token)
	require.NoError(t, err)
	require.Equal(t, "oidc", payload.Provider)
	require.Equal(t, "/dashboard", payload.ReturnURL)
	require.Equal(t, "nonce", payload.Nonce)
	require.Equal(t, "verifier", payload.PKCE)
	require.Equal(t, "request-123", payload.RequestID)
}

func TestStateCodecExpired(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	current := now
	codec, err := NewStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Minute, func() time.Time {
		return current
	})
	require.NoError(t, err)

	token, err := codec.Encode(StatePayload{Provider: "oidc", Nonce: "n", PKCE: "p"})
	require.NoError(t, err)

	current = current.Add(2 * time.Minute)
	_, err = codec.Decode(token)
	require.Error(t, err)
}
