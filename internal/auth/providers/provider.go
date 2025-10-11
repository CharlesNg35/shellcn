package providers

import (
	"context"
	"net/http"
)

// Metadata describes the static presentation details for an authentication provider.
type Metadata struct {
	Type          string
	DisplayName   string
	Description   string
	Icon          string
	ButtonText    string
	Order         int
	SupportsTest  bool
	SupportsLogin bool
	Flow          string
}

// BeginAuthRequest captures contextual information required to begin an external auth flow.
type BeginAuthRequest struct {
	State          string
	Nonce          string
	PKCEChallenge  string
	PKCEMethod     string
	RedirectURL    string
	SuccessURL     string
	ErrorURL       string
	Prompt         string
	RawHTTPRequest *http.Request
}

// BeginAuthResponse contains the redirect information required to continue the external auth flow.
type BeginAuthResponse struct {
	RedirectURL string
	State       string
	Headers     map[string]string
	RequestID   string
}

// CallbackRequest captures the raw HTTP details posted by an external provider.
type CallbackRequest struct {
	State          string
	PKCEVerifier   string
	ExpectedNonce  string
	AuthnRequestID string
	RawHTTPRequest *http.Request
}

// Identity represents the claims returned from an external authentication provider.
type Identity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
	DisplayName   string
	AvatarURL     string
	Groups        []string
	RawClaims     map[string]any
}

// Provider defines the behaviour required for an interactive external authentication provider.
type Provider interface {
	Metadata() Metadata
	Begin(ctx context.Context, req BeginAuthRequest) (*BeginAuthResponse, error)
	Callback(ctx context.Context, req CallbackRequest) (*Identity, error)
	Test(ctx context.Context) error
}

// MetadataProvider exposes a method to serialise provider-specific metadata documents.
type MetadataProvider interface {
	Provider
	ServiceProviderMetadata() ([]byte, error)
}
