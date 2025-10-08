package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	testutil "github.com/charlesng35/shellcn/internal/testutil"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

type stubOIDCProvider struct {
	identity providers.Identity
}

func (s *stubOIDCProvider) Metadata() providers.Metadata {
	return providers.Metadata{Type: "oidc", SupportsLogin: true}
}

func (s *stubOIDCProvider) Begin(ctx context.Context, req providers.BeginAuthRequest) (*providers.BeginAuthResponse, error) {
	redirect := "https://idp.example.com/auth?state=" + url.QueryEscape(req.State)
	return &providers.BeginAuthResponse{RedirectURL: redirect, State: req.State}, nil
}

func (s *stubOIDCProvider) Callback(ctx context.Context, req providers.CallbackRequest) (*providers.Identity, error) {
	return &s.identity, nil
}

func (s *stubOIDCProvider) Test(ctx context.Context) error { return nil }

func TestSSOHandlerFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := services.NewAuditService(db)
	require.NoError(t, err)

	encryptionKey := []byte("0123456789abcdef0123456789abcdef")

	authProviderSvc, err := services.NewAuthProviderService(db, auditSvc, encryptionKey)
	require.NoError(t, err)

	ctx := context.Background()

	require.NoError(t, db.Create(&models.AuthProvider{
		Type:                     "local",
		Name:                     "Local",
		Enabled:                  true,
		AllowRegistration:        true,
		RequireEmailVerification: true,
		Description:              "Local",
		Icon:                     "key",
	}).Error)

	err = authProviderSvc.ConfigureOIDC(ctx, models.OIDCConfig{
		Issuer:       "https://issuer.example.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://shellcn.example.com/api/auth/providers/oidc/callback",
		Scopes:       []string{"openid", "email"},
	}, true, "admin")
	require.NoError(t, err)

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{
		Secret:         "jwt-secret",
		AccessTokenTTL: time.Minute,
	})
	require.NoError(t, err)

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, iauth.SessionConfig{
		RefreshTokenTTL: time.Hour,
		RefreshLength:   32,
	})
	require.NoError(t, err)

	stateCodec, err := iauth.NewStateCodec(encryptionKey, time.Minute*5, nil)
	require.NoError(t, err)

	ssoManager, err := iauth.NewSSOManager(db, sessionSvc, iauth.SSOConfig{})
	require.NoError(t, err)

	stub := &stubOIDCProvider{identity: providers.Identity{
		Provider: "oidc",
		Subject:  "sub-123",
		Email:    "user@example.com",
	}}

	registry := providers.NewRegistry()
	require.NoError(t, registry.Register(providers.Descriptor{
		Metadata: providers.Metadata{Type: "oidc", SupportsLogin: true},
		Factory: func(cfg providers.ProviderConfig) (providers.Provider, error) {
			return stub, nil
		},
	}))

	handler := NewSSOHandler(registry, authProviderSvc, ssoManager, stateCodec)

	// Begin request
	beginRecorder := httptest.NewRecorder()
	beginCtx, _ := gin.CreateTestContext(beginRecorder)
	beginReq := httptest.NewRequest("GET", "/api/auth/providers/oidc/login?redirect=/app", nil)
	beginCtx.Request = beginReq
	beginCtx.Params = gin.Params{{Key: "type", Value: "oidc"}}

	handler.Begin(beginCtx)

	require.Equal(t, http.StatusFound, beginRecorder.Code)
	location := beginRecorder.Header().Get("Location")
	require.NotEmpty(t, location)

	parsedLocation, err := url.Parse(location)
	require.NoError(t, err)
	state := parsedLocation.Query().Get("state")
	require.NotEmpty(t, state)

	// Create local user to link identity
	hashed, err := crypto.HashPassword("password")
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.User{
		Username: "user",
		Email:    "user@example.com",
		Password: hashed,
		IsActive: true,
	}).Error)

	// Callback request
	callbackRecorder := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRecorder)
	callbackReq := httptest.NewRequest("GET", "/api/auth/providers/oidc/callback?state="+url.QueryEscape(state)+"&code=stub", nil)
	callbackCtx.Request = callbackReq
	callbackCtx.Params = gin.Params{{Key: "type", Value: "oidc"}}

	handler.Callback(callbackCtx)

	require.Equal(t, http.StatusSeeOther, callbackRecorder.Code)
	finalLocation := callbackRecorder.Header().Get("Location")
	require.NotEmpty(t, finalLocation)

	finalParsed, err := url.Parse(finalLocation)
	require.NoError(t, err)
	query := finalParsed.Query()
	require.NotEmpty(t, query.Get("access_token"))
	require.NotEmpty(t, query.Get("refresh_token"))
}
