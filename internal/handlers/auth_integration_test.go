package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/api"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestAuthLoginFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := database.Open(database.Config{Driver: "sqlite"})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrateAndSeed(db); err != nil {
		t.Fatalf("migrate/seed: %v", err)
	}

	// Create a test user
	hashed, _ := crypto.HashPassword("password123!")
	user := &models.User{Username: "alice", Email: "alice@example.com", Password: hashed, IsActive: true}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "test-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	router, err := api.NewRouter(db, jwtSvc)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	body := map[string]string{"identifier": "alice", "password": "password123!"}
	buf, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/auth/login", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 login, got %d: %s", w.Code, w.Body.String())
	}
}
