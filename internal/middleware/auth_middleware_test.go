package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	iauth "github.com/charlesng35/shellcn/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         "secret",
		Issuer:         "test-suite",
		AccessTokenTTL: time.Minute,
	})
	require.NoError(t, err)

	token, err := jwtSvc.GenerateAccessToken(iauth.AccessTokenInput{
		UserID:    "user-123",
		SessionID: "session-abc",
	})
	require.NoError(t, err)

	r := gin.New()
	r.GET("/secure", Auth(jwtSvc), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":    c.GetString(CtxUserIDKey),
			"session_id": c.GetString(CtxSessionIDKey),
		})
	})

	// Missing Authorization header -> 401
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Valid token -> downstream handler executes
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "user-123", payload["user_id"])
	require.Equal(t, "session-abc", payload["session_id"])
}
