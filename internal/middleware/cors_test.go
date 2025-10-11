package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORS())
	r.GET("/resource", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Preflight request
	preflight := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	r.ServeHTTP(preflight, req)
	require.Equal(t, http.StatusNoContent, preflight.Code)
	require.Equal(t, "*", preflight.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, preflight.Header().Get("Access-Control-Allow-Methods"), "GET")
	require.Contains(t, preflight.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	// Actual request inherits headers and proceeds
	w := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/resource", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
