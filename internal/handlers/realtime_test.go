package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/realtime"
)

func TestRealtimeHandlerUnauthorizedWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := realtime.NewHub()
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "test-secret"})
	require.NoError(t, err)

	handler := NewRealtimeHandler(hub, jwtSvc, nil, realtime.StreamNotifications)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{gin.Param{Key: "stream", Value: realtime.StreamNotifications}}
	c.Request = httptest.NewRequest(http.MethodGet, "/ws/notifications", nil)

	handler.Stream(c)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRealtimeHandlerRejectsUnknownStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := realtime.NewHub()
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "test-secret"})
	require.NoError(t, err)

	handler := NewRealtimeHandler(hub, jwtSvc, nil, realtime.StreamNotifications)

	token, err := jwtSvc.GenerateAccessToken(iauth.AccessTokenInput{UserID: "user-1"})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{gin.Param{Key: "stream", Value: "unknown"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/ws/unknown?token="+token, nil)

	handler.Stream(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
