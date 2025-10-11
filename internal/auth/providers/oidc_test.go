package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestNewOIDCDescriptorMetadata(t *testing.T) {
	desc := NewOIDCDescriptor(OIDCOptions{})
	meta := desc.Metadata

	if meta.Type != "oidc" {
		t.Fatalf("expected type oidc, got %s", meta.Type)
	}
	if !meta.SupportsLogin || !meta.SupportsTest {
		t.Fatalf("expected supports login+test flags: %+v", meta)
	}
	if meta.Flow != "redirect" {
		t.Fatalf("expected redirect flow, got %s", meta.Flow)
	}
}

func TestNewOIDCProviderValidatesType(t *testing.T) {
	_, err := newOIDCProvider(ProviderConfig{
		Type: "saml",
	}, OIDCOptions{})
	if err == nil {
		t.Fatal("expected error for unexpected provider type")
	}
}

func TestNewOIDCProviderRequiresFields(t *testing.T) {
	type testCase struct {
		name string
		cfg  models.OIDCConfig
		want string
	}

	cases := []testCase{
		{name: "issuer", cfg: models.OIDCConfig{}, want: "issuer is required"},
		{name: "client id", cfg: models.OIDCConfig{Issuer: "https://issuer"}, want: "client id is required"},
		{name: "client secret", cfg: models.OIDCConfig{Issuer: "https://issuer", ClientID: "abc"}, want: "client secret is required"},
		{name: "redirect url", cfg: models.OIDCConfig{Issuer: "https://issuer", ClientID: "abc", ClientSecret: "secret"}, want: "redirect url is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(tc.cfg)
			_, err := newOIDCProvider(ProviderConfig{
				Type: "oidc",
				Raw:  raw,
			}, OIDCOptions{})
			if err == nil || !contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNewOIDCProviderSuccessWithSecretOverride(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 server.URL,
				"authorization_endpoint": server.URL + "/auth",
				"token_endpoint":         server.URL + "/token",
				"jwks_uri":               server.URL + "/jwks",
			})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	raw, _ := json.Marshal(models.OIDCConfig{
		Issuer:      server.URL,
		ClientID:    "client-123",
		RedirectURL: "https://app.example.com/callback",
	})

	cfg := ProviderConfig{
		Type:        "oidc",
		Name:        "Acme OIDC",
		Description: "Custom desc",
		Icon:        "shield-custom",
		Raw:         raw,
		Secrets: map[string]string{
			"client_secret": "super-secret",
		},
	}

	opts := OIDCOptions{
		HTTPClient: server.Client(),
		Timeout:    time.Second,
		Now:        time.Now,
	}

	provider, err := newOIDCProvider(cfg, opts)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	oidcProv, ok := provider.(*oidcProvider)
	if !ok {
		t.Fatalf("expected *oidcProvider, got %T", provider)
	}

	if oidcProv.oauthConfig.ClientSecret != "super-secret" {
		t.Fatalf("expected secret override to apply, got %q", oidcProv.oauthConfig.ClientSecret)
	}
	meta := oidcProv.Metadata()
	if meta.DisplayName != "Acme OIDC" {
		t.Fatalf("metadata display name mismatch: %s", meta.DisplayName)
	}
	if meta.Icon != "shield-custom" {
		t.Fatalf("metadata icon mismatch: %s", meta.Icon)
	}
}

func contains(haystack, needle string) bool {
	return needle == "" || (haystack != "" && strings.Contains(haystack, needle))
}
