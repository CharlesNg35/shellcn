package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCSRFIssuesTokenOnSafeMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRF())
	r.GET("/status", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	resp := w.Result()
	defer resp.Body.Close()

	var csrfCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == CSRFCookieName {
			csrfCookie = c
			break
		}
	}
	require.NotNil(t, csrfCookie)
	require.NotEmpty(t, csrfCookie.Value)

	headerToken := resp.Header.Get(CSRFHeaderName)
	require.NotEmpty(t, headerToken)
	require.Equal(t, csrfCookie.Value, headerToken)
}

func TestCSRFAcceptsValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRF())
	r.POST("/submit", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// Get token
	tokenResp := httptest.NewRecorder()
	tokenReq := httptest.NewRequest(http.MethodGet, "/submit", nil)
	r.ServeHTTP(tokenResp, tokenReq)
	resp := tokenResp.Result()
	defer resp.Body.Close()

	var csrfCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == CSRFCookieName {
			csrfCookie = c
		}
	}
	require.NotNil(t, csrfCookie)
	token := resp.Header.Get(CSRFHeaderName)
	require.NotEmpty(t, token)

	// POST with valid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.AddCookie(csrfCookie)
	req.Header.Set(CSRFHeaderName, token)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestCSRFFailsWithMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CSRF())
	r.POST("/update", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}
