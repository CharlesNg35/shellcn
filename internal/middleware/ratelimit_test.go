package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RateLimit(2, 100*time.Millisecond))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })

	// First two requests should pass
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", nil)
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}

	// Third request within window should be rate-limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	time.Sleep(120 * time.Millisecond)

	// After window resets, should pass again
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 after reset, got %d", w.Code)
	}
}
