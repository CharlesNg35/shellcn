package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/api"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/database"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/response"
)

// Env encapsulates a fully-wired API instance backed by an in-memory database for handler tests.
type Env struct {
	T      *testing.T
	DB     *gorm.DB
	Router *gin.Engine
	JWT    *iauth.JWTService
}

// NewEnv provisions a fresh handler test environment with migrations and seed data applied.
func NewEnv(t *testing.T) *Env {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := database.Open(database.Config{Driver: "sqlite"})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrateAndSeed(db))

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         "test-secret",
		Issuer:         "test-suite",
		AccessTokenTTL: time.Hour,
	})
	require.NoError(t, err)

	router, err := api.NewRouter(db, jwtSvc)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	return &Env{
		T:      t,
		DB:     db,
		Router: router,
		JWT:    jwtSvc,
	}
}

// CreateRootUser inserts a new active root user with a random username and returns the record.
func (e *Env) CreateRootUser(password string) *models.User {
	e.T.Helper()

	username := "root-" + uuid.NewString()
	hashed, err := crypto.HashPassword(password)
	require.NoError(e.T, err)

	user := &models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: hashed,
		IsActive: true,
		IsRoot:   true,
	}

	require.NoError(e.T, e.DB.Create(user).Error)
	return user
}

// TokenPair mirrors the handler login response payload.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// UserPayload captures the subset of user fields returned from auth endpoints.
type UserPayload struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsRoot    bool   `json:"is_root"`
	IsActive  bool   `json:"is_active"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginResult bundles the JSON response from POST /api/auth/login.
type LoginResult struct {
	Tokens      TokenPair   `json:"tokens"`
	User        UserPayload `json:"user"`
	Permissions []string    `json:"permissions"`
}

// Login authenticates using the local provider and returns the issued token pair.
func (e *Env) Login(username, password string) LoginResult {
	e.T.Helper()

	payload := map[string]string{
		"identifier": username,
		"password":   password,
	}

	w := e.Request(http.MethodPost, "/api/auth/login", payload, "")
	require.Equal(e.T, http.StatusOK, w.Code, w.Body.String())

	resp := DecodeResponse(e.T, w)
	require.True(e.T, resp.Success, w.Body.String())

	var result LoginResult
	DecodeInto(e.T, resp.Data, &result)
	require.NotEmpty(e.T, result.Tokens.AccessToken)
	require.NotEmpty(e.T, result.Tokens.RefreshToken)
	require.Equal(e.T, username, result.User.Username)

	return result
}

// APIResponse represents the canonical API envelope returned by handlers.
type APIResponse struct {
	Success bool                `json:"success"`
	Data    json.RawMessage     `json:"data"`
	Error   *response.ErrorInfo `json:"error"`
	Meta    *response.Meta      `json:"meta"`
}

// DecodeResponse parses the standard API response object from a recorder.
func DecodeResponse(t *testing.T, w *httptest.ResponseRecorder) APIResponse {
	t.Helper()
	var resp APIResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
	return resp
}

// DecodeInto unmarshals the data payload into the provided destination.
func DecodeInto[T any](t *testing.T, raw json.RawMessage, dest *T) {
	t.Helper()
	if dest == nil {
		t.Fatal("destination must not be nil")
	}
	require.NoError(t, json.Unmarshal(raw, dest))
}

// Request executes an HTTP request against the test router, applying JSON encoding and auth headers automatically.
func (e *Env) Request(method, path string, body any, token string) *httptest.ResponseRecorder {
	e.T.Helper()

	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(e.T, err)
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequest(method, path, buf)
	require.NoError(e.T, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	e.Router.ServeHTTP(w, req)
	return w
}
