package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	testutil "github.com/charlesng35/shellcn/internal/testutil"
)

func TestRouter_PublicAndProtectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Open in-memory DB and run migrations/seed
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "test-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	router, err := NewRouter(db, jwtSvc)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	// Health should be public
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for /health, got %d", w.Code)
	}

	// Protected endpoint without auth should be 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/auth/me", nil)
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 for /api/auth/me without token, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 for /api/users without token, got %d", w.Code)
	}
}
