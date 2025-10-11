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
	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	sharedtestutil "github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/response"
)

// Env encapsulates a fully-wired API instance backed by an in-memory database for handler tests.
type Env struct {
	T          *testing.T
	DB         *gorm.DB
	Router     *gin.Engine
	JWT        *iauth.JWTService
	csrfToken  string
	csrfCookie *http.Cookie
}

// NewEnv provisions a fresh handler test environment with migrations and seed data applied.
func NewEnv(t *testing.T) *Env {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db := sharedtestutil.MustOpenTestDB(t, sharedtestutil.WithSeedData())

	jwtSecret := "test-suite-super-secret-key-32-bytes!!"
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         jwtSecret,
		Issuer:         "test-suite",
		AccessTokenTTL: time.Hour,
	})
	require.NoError(t, err)

	cfg := &app.Config{
		Vault: app.VaultConfig{
			EncryptionKey: "0123456789abcdef0123456789abcdef",
		},
		Auth: app.AuthConfig{
			JWT: app.JWTSettings{
				Secret: jwtSecret,
				Issuer: "test-suite",
				TTL:    time.Hour,
			},
			Session: app.SessionSettings{
				RefreshTTL:    24 * time.Hour,
				RefreshLength: 48,
			},
		},
	}

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	require.NoError(t, err)

	router, err := api.NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore())
	require.NoError(t, err)

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

	var roles []models.Role
	require.NoError(e.T, e.DB.Where("id IN ?", []string{"admin", "user"}).Find(&roles).Error)
	require.Len(e.T, roles, 2)
	roleInterfaces := make([]any, len(roles))
	for i := range roles {
		roleInterfaces[i] = &roles[i]
	}
	require.NoError(e.T, e.DB.Model(user).Association("Roles").Append(roleInterfaces...))
	return user
}

// TokenPair mirrors the handler login response payload.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// UserPayload captures the subset of user fields returned from auth endpoints.
type UserPayload struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	Email       string        `json:"email"`
	IsRoot      bool          `json:"is_root"`
	IsActive    bool          `json:"is_active"`
	FirstName   string        `json:"first_name"`
	LastName    string        `json:"last_name"`
	Permissions []string      `json:"permissions"`
	Roles       []RolePayload `json:"roles"`
}

type RolePayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// LoginResult bundles the JSON response from POST /api/auth/login.
type LoginResult struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int         `json:"expires_in"`
	User         UserPayload `json:"user"`
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
	require.NotEmpty(e.T, result.AccessToken)
	require.NotEmpty(e.T, result.RefreshToken)
	require.Greater(e.T, result.ExpiresIn, 0)
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
	return e.request(method, path, body, token, false)
}

func (e *Env) request(method, path string, body any, token string, skipCSRF bool) *httptest.ResponseRecorder {
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

	if !skipCSRF && requiresCSRFAttestation(method) {
		e.ensureCSRFToken()
		if e.csrfCookie != nil {
			req.AddCookie(e.csrfCookie)
		}
		if e.csrfToken != "" {
			req.Header.Set(middleware.CSRFHeaderName, e.csrfToken)
		}
	}

	w := httptest.NewRecorder()
	e.Router.ServeHTTP(w, req)

	e.captureCSRF(w.Result())
	return w
}

func (e *Env) ensureCSRFToken() {
	if e.csrfToken != "" && e.csrfCookie != nil {
		return
	}
	resp := e.request(http.MethodGet, "/health", nil, "", true)
	require.Equal(e.T, http.StatusOK, resp.Code, resp.Body.String())
}

func (e *Env) captureCSRF(resp *http.Response) {
	if resp == nil {
		return
	}
	defer resp.Body.Close()

	if token := resp.Header.Get(middleware.CSRFHeaderName); token != "" {
		e.csrfToken = token
	}
	for _, c := range resp.Cookies() {
		if c.Name == middleware.CSRFCookieName {
			// Clone to avoid unintended mutations between tests
			e.csrfCookie = &http.Cookie{
				Name:       c.Name,
				Value:      c.Value,
				Path:       c.Path,
				Domain:     c.Domain,
				Expires:    c.Expires,
				Raw:        c.Raw,
				MaxAge:     c.MaxAge,
				Secure:     c.Secure,
				HttpOnly:   c.HttpOnly,
				SameSite:   c.SameSite,
				RawExpires: c.RawExpires,
			}
			break
		}
	}
}

func requiresCSRFAttestation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
