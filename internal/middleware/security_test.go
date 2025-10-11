package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	resp := w.Result()
	defer resp.Body.Close()

	require.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	require.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	require.Equal(t, "max-age=31536000; includeSubDomains", resp.Header.Get("Strict-Transport-Security"))
	require.Contains(t, resp.Header.Get("Content-Security-Policy"), "default-src 'self'")
	require.Contains(t, resp.Header.Get("Content-Security-Policy"), "font-src 'self' https://fonts.gstatic.com")
	require.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
	require.Equal(t, "geolocation=(), microphone=(), camera=()", resp.Header.Get("Permissions-Policy"))
}
