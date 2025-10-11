package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/pkg/response"
)

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var payload response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Equal(t, "INTERNAL_SERVER_ERROR", payload.Error.Code)
}

func TestNotFoundHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.NoRoute(NotFoundHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var payload response.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	data := payload.Data.(map[string]any)
	require.Contains(t, data["error"].(string), "route /missing not found")
}
