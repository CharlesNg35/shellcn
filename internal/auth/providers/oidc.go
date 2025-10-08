package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/charlesng35/shellcn/internal/models"
)

// OIDCOptions configures the behaviour of the OIDC provider implementation.
type OIDCOptions struct {
	HTTPClient *http.Client
	Now        func() time.Time
	Timeout    time.Duration
}

// NewOIDCDescriptor registers the OIDC provider implementation with the supplied options.
func NewOIDCDescriptor(opts OIDCOptions) Descriptor {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	return Descriptor{
		Metadata: Metadata{
			Type:          "oidc",
			DisplayName:   "OpenID Connect",
			Description:   "Single Sign-On via OIDC",
			Icon:          "shield-check",
			ButtonText:    "Continue with SSO",
			SupportsTest:  true,
			SupportsLogin: true,
			Order:         10,
			Flow:          "redirect",
		},
		Factory: func(cfg ProviderConfig) (Provider, error) {
			return newOIDCProvider(cfg, opts)
		},
	}
}

type oidcProvider struct {
	metadata    Metadata
	oauthConfig *oauth2.Config
	verifier    *oidc.IDTokenVerifier
	now         func() time.Time
	timeout     time.Duration
}

func newOIDCProvider(cfg ProviderConfig, opts OIDCOptions) (Provider, error) {
	if strings.TrimSpace(cfg.Type) != "oidc" {
		return nil, fmt.Errorf("oidc provider: unexpected type %s", cfg.Type)
	}

	var rawCfg models.OIDCConfig
	if err := json.Unmarshal(cfg.Raw, &rawCfg); err != nil {
		return nil, fmt.Errorf("oidc provider: decode config: %w", err)
	}

	secret := strings.TrimSpace(cfg.Secrets["client_secret"])
	if secret != "" {
		rawCfg.ClientSecret = secret
	}

	if strings.TrimSpace(rawCfg.Issuer) == "" {
		return nil, errors.New("oidc provider: issuer is required")
	}
	if strings.TrimSpace(rawCfg.ClientID) == "" {
		return nil, errors.New("oidc provider: client id is required")
	}
	if strings.TrimSpace(rawCfg.ClientSecret) == "" {
		return nil, errors.New("oidc provider: client secret is required")
	}
	if strings.TrimSpace(rawCfg.RedirectURL) == "" {
		return nil, errors.New("oidc provider: redirect url is required")
	}

	scopes := rawCfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}

	ctx := context.Background()
	if opts.HTTPClient != nil {
		ctx = oidc.ClientContext(ctx, opts.HTTPClient)
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	issuer, err := oidc.NewProvider(ctx, rawCfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: discovery failed: %w", err)
	}

	oauthConfig := &oauth2.Config{
		ClientID:     rawCfg.ClientID,
		ClientSecret: rawCfg.ClientSecret,
		Endpoint:     issuer.Endpoint(),
		RedirectURL:  rawCfg.RedirectURL,
		Scopes:       scopes,
	}

	verifier := issuer.Verifier(&oidc.Config{ClientID: rawCfg.ClientID})

	displayName := cfg.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = "OpenID Connect"
	}
	description := cfg.Description
	if strings.TrimSpace(description) == "" {
		description = displayName
	}

	icon := cfg.Icon
	if strings.TrimSpace(icon) == "" {
		icon = "shield-check"
	}

	return &oidcProvider{
		metadata: Metadata{
			Type:          cfg.Type,
			DisplayName:   displayName,
			Description:   description,
			Icon:          icon,
			ButtonText:    "Continue with SSO",
			SupportsTest:  true,
			SupportsLogin: true,
			Order:         10,
			Flow:          "redirect",
		},
		oauthConfig: oauthConfig,
		verifier:    verifier,
		now:         opts.Now,
		timeout:     opts.Timeout,
	}, nil
}

func (p *oidcProvider) Metadata() Metadata {
	return p.metadata
}

func (p *oidcProvider) Begin(ctx context.Context, req BeginAuthRequest) (*BeginAuthResponse, error) {
	if strings.TrimSpace(req.State) == "" {
		return nil, errors.New("oidc provider: state is required")
	}
	if strings.TrimSpace(req.Nonce) == "" {
		return nil, errors.New("oidc provider: nonce is required")
	}
	if strings.TrimSpace(req.PKCEChallenge) == "" {
		return nil, errors.New("oidc provider: pkce challenge is required")
	}

	authOpts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("nonce", req.Nonce),
		oauth2.SetAuthURLParam("code_challenge", req.PKCEChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	if req.Prompt != "" {
		authOpts = append(authOpts, oauth2.SetAuthURLParam("prompt", req.Prompt))
	}

	url := p.oauthConfig.AuthCodeURL(req.State, authOpts...)
	return &BeginAuthResponse{RedirectURL: url, State: req.State}, nil
}

func (p *oidcProvider) Callback(ctx context.Context, req CallbackRequest) (*Identity, error) {
	if req.RawHTTPRequest == nil {
		return nil, errors.New("oidc provider: request is required")
	}
	query := req.RawHTTPRequest.URL.Query()
	if errStr := query.Get("error"); errStr != "" {
		return nil, fmt.Errorf("oidc provider: authorization error: %s", errStr)
	}
	code := query.Get("code")
	if code == "" {
		return nil, errors.New("oidc provider: authorization code missing")
	}
	if strings.TrimSpace(req.PKCEVerifier) == "" {
		return nil, errors.New("oidc provider: pkce verifier is required")
	}

	tokenCtx := ctx
	if tokenCtx == nil {
		tokenCtx = context.Background()
	}
	tokenCtx, cancel := context.WithTimeout(tokenCtx, p.timeout)
	defer cancel()

	token, err := p.oauthConfig.Exchange(tokenCtx, code, oauth2.SetAuthURLParam("code_verifier", req.PKCEVerifier))
	if err != nil {
		return nil, fmt.Errorf("oidc provider: exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("oidc provider: id token missing")
	}

	idToken, err := p.verifier.Verify(tokenCtx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: verify id token: %w", err)
	}
	if req.ExpectedNonce != "" && idToken.Nonce != req.ExpectedNonce {
		return nil, errors.New("oidc provider: nonce mismatch")
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc provider: decode claims: %w", err)
	}

	identity := &Identity{
		Provider:      "oidc",
		Subject:       idToken.Subject,
		Email:         stringValue(claims, "email"),
		EmailVerified: boolValue(claims, "email_verified"),
		FirstName:     stringValue(claims, "given_name"),
		LastName:      stringValue(claims, "family_name"),
		DisplayName:   stringValue(claims, "name"),
		AvatarURL:     stringValue(claims, "picture"),
		RawClaims:     claims,
	}

	if groups, ok := claims["groups"].([]any); ok {
		identity.Groups = extractStringSlice(groups)
	}

	return identity, nil
}

func (p *oidcProvider) Test(ctx context.Context) error {
	// Discovery performed during construction. An additional test can simply verify endpoint reachability.
	if p.oauthConfig == nil || p.verifier == nil {
		return errors.New("oidc provider: not initialised")
	}
	return nil
}

func stringValue(claims map[string]any, key string) string {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func boolValue(claims map[string]any, key string) bool {
	if v, ok := claims[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return strings.EqualFold(val, "true")
		}
	}
	return false
}

func extractStringSlice(values []any) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
