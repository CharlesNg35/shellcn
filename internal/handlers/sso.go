package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/crypto"
)

// SSOHandler manages external authentication login and callback flows.
type SSOHandler struct {
	registry   *providers.Registry
	svc        *services.AuthProviderService
	manager    *iauth.SSOManager
	stateCodec *iauth.StateCodec
}

func NewSSOHandler(registry *providers.Registry, svc *services.AuthProviderService, manager *iauth.SSOManager, codec *iauth.StateCodec) *SSOHandler {
	return &SSOHandler{registry: registry, svc: svc, manager: manager, stateCodec: codec}
}

// Begin initiates the external authentication flow by redirecting the user to the provider's authorization endpoint.
func (h *SSOHandler) Begin(c *gin.Context) {
	providerType := strings.ToLower(strings.TrimSpace(c.Param("type")))
	if providerType == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	provider, autoProvision, err := h.instantiateProvider(requestContext(c), providerType)
	if err != nil {
		handleSSOError(c, err)
		return
	}

	pkce, err := iauth.GeneratePKCE()
	if err != nil {
		handleSSOError(c, err)
		return
	}

	nonceToken, err := crypto.GenerateToken(32)
	if err != nil {
		handleSSOError(c, err)
		return
	}

	statePayload := iauth.StatePayload{
		Provider:   providerType,
		ReturnURL:  sanitizeRedirect(c.Query("redirect"), "/"),
		ErrorURL:   sanitizeRedirect(c.Query("error_redirect"), "/login?error=sso_failed"),
		Nonce:      nonceToken,
		PKCE:       pkce.Verifier,
		AutoCreate: autoProvision,
	}

	state, err := h.stateCodec.Encode(statePayload)
	if err != nil {
		handleSSOError(c, err)
		return
	}

	resp, err := provider.Begin(requestContext(c), providers.BeginAuthRequest{
		State:          state,
		Nonce:          nonceToken,
		PKCEChallenge:  pkce.Challenge,
		PKCEMethod:     "S256",
		RawHTTPRequest: c.Request,
	})
	if err != nil {
		handleSSOError(c, err)
		return
	}

	if resp.RequestID != "" {
		statePayload.RequestID = resp.RequestID
		state, err = h.stateCodec.Encode(statePayload)
		if err != nil {
			handleSSOError(c, err)
			return
		}
		resp.RedirectURL = replaceStateQuery(resp.RedirectURL, state)
	}

	if resp.State == "" {
		resp.State = state
	}

	for key, value := range resp.Headers {
		c.Header(key, value)
	}
	c.Redirect(http.StatusFound, resp.RedirectURL)
}

// Callback processes the provider callback, issues a session and redirects back to the application.
func (h *SSOHandler) Callback(c *gin.Context) {
	stateToken := c.Query("state")
	payload, err := h.stateCodec.Decode(stateToken)
	if err != nil {
		redirectWithError(c, "/login?error=sso_state", err)
		return
	}

	provider, _, err := h.instantiateProvider(requestContext(c), payload.Provider)
	if err != nil {
		redirectWithError(c, payload.ErrorURL, err)
		return
	}

	identity, err := provider.Callback(requestContext(c), providers.CallbackRequest{
		State:          stateToken,
		PKCEVerifier:   payload.PKCE,
		ExpectedNonce:  payload.Nonce,
		AuthnRequestID: payload.RequestID,
		RawHTTPRequest: c.Request,
	})
	if err != nil {
		redirectWithError(c, payload.ErrorURL, err)
		return
	}

	tokens, user, _, err := h.manager.Resolve(requestContext(c), *identity, iauth.ResolveOptions{
		AutoProvision: payload.AutoCreate,
		SessionMeta: iauth.SessionMetadata{
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		},
	})
	if err != nil {
		redirectWithError(c, payload.ErrorURL, err)
		return
	}

	redirectURL := appendTokens(payload.ReturnURL, tokens, user)
	c.Redirect(http.StatusSeeOther, redirectURL)
}

// Metadata exposes provider metadata documents (e.g., SAML SP metadata).
func (h *SSOHandler) Metadata(c *gin.Context) {
	providerType := strings.ToLower(strings.TrimSpace(c.Param("type")))
	if providerType == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "provider is required"})
		return
	}

	provider, _, err := h.instantiateProvider(requestContext(c), providerType)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	metadataProvider, ok := provider.(providers.MetadataProvider)
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "metadata not supported"})
		return
	}

	data, err := metadataProvider.ServiceProviderMetadata()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/samlmetadata+xml", data)
}

func (h *SSOHandler) instantiateProvider(ctx context.Context, providerType string) (providers.Provider, bool, error) {
	factory, ok := h.registry.FactoryFor(providerType)
	if !ok {
		return nil, false, errors.New("provider not supported")
	}

	switch providerType {
	case "oidc":
		providerModel, cfg, err := h.svc.LoadOIDCConfig(ctx)
		if err != nil {
			return nil, false, err
		}
		if !providerModel.Enabled {
			return nil, false, errors.New("provider disabled")
		}

		copyCfg := *cfg
		secret := copyCfg.ClientSecret
		copyCfg.ClientSecret = ""

		raw, err := json.Marshal(copyCfg)
		if err != nil {
			return nil, false, err
		}

		instance, err := factory(providers.ProviderConfig{
			Type:        providerModel.Type,
			Name:        providerModel.Name,
			Description: providerModel.Description,
			Icon:        providerModel.Icon,
			Enabled:     providerModel.Enabled,
			Raw:         raw,
			Secrets:     map[string]string{"client_secret": secret},
		})
		return instance, providerModel.AllowRegistration, err
	case "saml":
		providerModel, cfg, err := h.svc.LoadSAMLConfig(ctx)
		if err != nil {
			return nil, false, err
		}
		if !providerModel.Enabled {
			return nil, false, errors.New("provider disabled")
		}

		copyCfg := *cfg
		secret := copyCfg.PrivateKey
		copyCfg.PrivateKey = ""

		raw, err := json.Marshal(copyCfg)
		if err != nil {
			return nil, false, err
		}

		instance, err := factory(providers.ProviderConfig{
			Type:        providerModel.Type,
			Name:        providerModel.Name,
			Description: providerModel.Description,
			Icon:        providerModel.Icon,
			Enabled:     providerModel.Enabled,
			Raw:         raw,
			Secrets:     map[string]string{"private_key": secret},
		})
		return instance, providerModel.AllowRegistration, err
	default:
		return nil, false, errors.New("provider not implemented")
	}
}

func sanitizeRedirect(input, fallback string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return fallback
	}

	if strings.Contains(trimmed, "\n") || strings.Contains(trimmed, "\r") {
		return fallback
	}

	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}

	return fallback
}

func appendTokens(redirect string, tokens iauth.TokenPair, user *models.User) string {
	parsed, err := url.Parse(redirect)
	if err != nil || parsed.Scheme == "" && !strings.HasPrefix(redirect, "/") {
		parsed = &url.URL{Path: redirect}
	}

	q := parsed.Query()
	q.Set("access_token", tokens.AccessToken)
	q.Set("refresh_token", tokens.RefreshToken)
	if user != nil {
		q.Set("user_id", user.ID)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func handleSSOError(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func redirectWithError(c *gin.Context, target string, err error) {
	if target == "" {
		target = "/login?error=sso_failed"
	}

	parsed, parseErr := url.Parse(target)
	if parseErr != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q := parsed.Query()
	q.Set("error", "sso_failed")
	switch {
	case errors.Is(err, iauth.ErrSSOProviderMismatch):
		q.Set("error_reason", "provider_mismatch")
	case errors.Is(err, iauth.ErrSSOUserNotFound):
		q.Set("error_reason", "not_found")
	case errors.Is(err, iauth.ErrSSOEmailRequired):
		q.Set("error_reason", "email_required")
	case errors.Is(err, iauth.ErrSSOUserDisabled):
		q.Set("error_reason", "user_disabled")
	default:
		q.Set("error_reason", "generic")
	}
	parsed.RawQuery = q.Encode()
	c.Redirect(http.StatusSeeOther, parsed.String())
}

func replaceStateQuery(rawURL, state string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
