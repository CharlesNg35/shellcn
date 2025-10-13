package providers

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	saml "github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"

	"github.com/charlesng35/shellcn/internal/models"
)

// SAMLOptions defines optional dependencies for building a SAML provider.
type SAMLOptions struct {
	HTTPClient *http.Client
	Now        func() time.Time
	Timeout    time.Duration
}

// NewSAMLDescriptor registers the SAML2 provider implementation.
func NewSAMLDescriptor(opts SAMLOptions) Descriptor {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	return Descriptor{
		Metadata: Metadata{
			Type:          "saml",
			DisplayName:   "SAML 2.0",
			Description:   "SAML 2.0 Single Sign-On",
			Icon:          "shield",
			ButtonText:    "Continue with SAML",
			SupportsTest:  true,
			SupportsLogin: true,
			Order:         20,
			Flow:          "redirect",
		},
		Factory: func(cfg ProviderConfig) (Provider, error) {
			return newSAMLProvider(cfg, opts)
		},
	}
}

type samlProvider struct {
	metadata Metadata
	sp       *saml.ServiceProvider
	attrMap  map[string]string
	now      func() time.Time
}

func newSAMLProvider(cfg ProviderConfig, opts SAMLOptions) (Provider, error) {
	if strings.TrimSpace(cfg.Type) != "saml" {
		return nil, fmt.Errorf("saml provider: unexpected type %s", cfg.Type)
	}

	var rawCfg models.SAMLConfig
	if err := json.Unmarshal(cfg.Raw, &rawCfg); err != nil {
		return nil, fmt.Errorf("saml provider: decode config: %w", err)
	}

	if secret := strings.TrimSpace(cfg.Secrets["private_key"]); secret != "" {
		rawCfg.PrivateKey = secret
	}

	if strings.TrimSpace(rawCfg.EntityID) == "" {
		return nil, errors.New("saml provider: entity id is required")
	}
	if strings.TrimSpace(rawCfg.ACSURL) == "" {
		return nil, errors.New("saml provider: acs url is required")
	}

	privateKey, err := parsePrivateKey(rawCfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("saml provider: parse private key: %w", err)
	}

	certificate, intermediates, err := parseCertificateChain(rawCfg.Certificate)
	if err != nil {
		return nil, fmt.Errorf("saml provider: parse certificate: %w", err)
	}

	acsURL, err := url.Parse(rawCfg.ACSURL)
	if err != nil {
		return nil, fmt.Errorf("saml provider: parse acs url: %w", err)
	}

	metadataURL := *acsURL
	metadataURL.Path = ensureTrailingPath(metadataURL.Path, "/metadata")

	sp := &saml.ServiceProvider{
		EntityID:      rawCfg.EntityID,
		Key:           privateKey,
		Certificate:   certificate,
		MetadataURL:   metadataURL,
		AcsURL:        *acsURL,
		Intermediates: intermediates,
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	if err := populateIDPMetadata(httpClient, rawCfg, sp, opts.Timeout); err != nil {
		return nil, err
	}

	attrMap := normalizeAttributeMap(rawCfg.AttributeMapping)

	displayName := cfg.Name
	if strings.TrimSpace(displayName) == "" {
		displayName = "SAML 2.0"
	}
	description := cfg.Description
	if strings.TrimSpace(description) == "" {
		description = displayName
	}

	icon := cfg.Icon
	if strings.TrimSpace(icon) == "" {
		icon = "shield"
	}

	return &samlProvider{
		metadata: Metadata{
			Type:          cfg.Type,
			DisplayName:   displayName,
			Description:   description,
			Icon:          icon,
			ButtonText:    "Continue with SAML",
			SupportsTest:  true,
			SupportsLogin: true,
			Order:         20,
			Flow:          "redirect",
		},
		sp:      sp,
		attrMap: attrMap,
		now:     opts.Now,
	}, nil
}

func (p *samlProvider) Metadata() Metadata {
	return p.metadata
}

func (p *samlProvider) Begin(ctx context.Context, req BeginAuthRequest) (*BeginAuthResponse, error) {
	relayState := req.State
	if strings.TrimSpace(relayState) == "" {
		return nil, errors.New("saml provider: state is required")
	}

	authnReq, err := p.sp.MakeAuthenticationRequest(p.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding), saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	if err != nil {
		return nil, fmt.Errorf("saml provider: make auth request: %w", err)
	}

	redirectURL, err := authnReq.Redirect(relayState, p.sp)
	if err != nil {
		return nil, fmt.Errorf("saml provider: build redirect: %w", err)
	}

	return &BeginAuthResponse{
		RedirectURL: redirectURL.String(),
		State:       relayState,
		RequestID:   authnReq.ID,
	}, nil
}

func (p *samlProvider) Callback(ctx context.Context, req CallbackRequest) (*Identity, error) {
	if req.RawHTTPRequest == nil {
		return nil, errors.New("saml provider: request is required")
	}
	if strings.TrimSpace(req.AuthnRequestID) == "" {
		return nil, errors.New("saml provider: request id missing")
	}

	assertion, err := p.sp.ParseResponse(req.RawHTTPRequest, []string{req.AuthnRequestID})
	if err != nil {
		return nil, fmt.Errorf("saml provider: parse response: %w", err)
	}

	attrs := collectAttributes(assertion)
	identity := &Identity{
		Provider:      "saml",
		Subject:       assertion.Subject.NameID.Value,
		Email:         attributeLookup(attrs, p.attrMap["email"]),
		EmailVerified: true,
		FirstName:     attributeLookup(attrs, p.attrMap["first_name"]),
		LastName:      attributeLookup(attrs, p.attrMap["last_name"]),
		DisplayName:   attributeLookup(attrs, p.attrMap["display_name"]),
		AvatarURL:     attributeLookup(attrs, p.attrMap["avatar"]),
		RawClaims:     make(map[string]any),
	}

	if identity.DisplayName == "" {
		identity.DisplayName = attributeLookup(attrs, "displayName")
	}
	if identity.Email == "" {
		identity.Email = attributeLookup(attrs, "email")
	}

	if groupsAttr := p.attrMap["groups"]; groupsAttr != "" {
		identity.Groups = attributeValues(attrs, groupsAttr)
	}

	for k, v := range attrs {
		values := make([]string, len(v))
		copy(values, v)
		identity.RawClaims[k] = values
	}

	return identity, nil
}

func (p *samlProvider) Test(ctx context.Context) error {
	if p.sp == nil || p.sp.IDPMetadata == nil {
		return errors.New("saml provider: metadata not initialised")
	}
	return nil
}

func (p *samlProvider) ServiceProviderMetadata() ([]byte, error) {
	meta := p.sp.Metadata()
	return xml.MarshalIndent(meta, "", "  ")
}

func populateIDPMetadata(httpClient *http.Client, cfg models.SAMLConfig, sp *saml.ServiceProvider, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := httpClient
	if client == nil {
		client = http.DefaultClient
	}

	if strings.TrimSpace(cfg.MetadataURL) != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.MetadataURL, nil)
		if err != nil {
			return fmt.Errorf("saml provider: build metadata request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("saml provider: fetch metadata: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("saml provider: metadata fetch failed: %s", resp.Status)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("saml provider: read metadata: %w", err)
		}
		entity, err := samlsp.ParseMetadata(data)
		if err != nil {
			return fmt.Errorf("saml provider: parse metadata: %w", err)
		}
		sp.IDPMetadata = entity
		return nil
	}

	if strings.TrimSpace(cfg.SSOURL) == "" {
		return errors.New("saml provider: sso url required when metadata url not provided")
	}

	signingCert := sp.Certificate
	if signingCert == nil {
		var parseErr error
		signingCert, _, parseErr = parseCertificateChain(cfg.Certificate)
		if parseErr != nil {
			return fmt.Errorf("saml provider: parse idp certificate: %w", parseErr)
		}
	}
	certData := base64.StdEncoding.EncodeToString(signingCert.Raw)

	sp.IDPMetadata = &saml.EntityDescriptor{
		EntityID: cfg.SSOURL,
		IDPSSODescriptors: []saml.IDPSSODescriptor{{
			SSODescriptor: saml.SSODescriptor{
				RoleDescriptor: saml.RoleDescriptor{
					KeyDescriptors: []saml.KeyDescriptor{{
						Use:     "signing",
						KeyInfo: saml.KeyInfo{X509Data: saml.X509Data{X509Certificates: []saml.X509Certificate{{Data: certData}}}},
					}},
				},
			},
			SingleSignOnServices: []saml.Endpoint{
				{Binding: saml.HTTPRedirectBinding, Location: cfg.SSOURL},
				{Binding: saml.HTTPPostBinding, Location: cfg.SSOURL},
			},
		}},
	}

	return nil
}

func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("saml provider: invalid private key pem")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("saml provider: private key must be RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("saml provider: unsupported private key type %s", block.Type)
	}
}

func parseCertificateChain(pemData string) (*x509.Certificate, []*x509.Certificate, error) {
	var (
		primary       *x509.Certificate
		intermediates []*x509.Certificate
	)

	rest := []byte(pemData)
	for {
		if len(strings.TrimSpace(string(rest))) == 0 {
			break
		}
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		if primary == nil {
			primary = cert
		} else {
			intermediates = append(intermediates, cert)
		}
	}

	if primary == nil {
		return nil, nil, errors.New("saml provider: certificate not found")
	}
	return primary, intermediates, nil
}

func ensureTrailingPath(path string, suffix string) string {
	if strings.HasSuffix(path, suffix) {
		return path
	}
	if strings.HasSuffix(path, "/") {
		return path + strings.TrimPrefix(suffix, "/")
	}
	return path + suffix
}

func normalizeAttributeMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	return out
}

func collectAttributes(assertion *saml.Assertion) map[string][]string {
	result := make(map[string][]string)
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			name := attr.FriendlyName
			if name == "" {
				name = attr.Name
			}
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			for _, v := range attr.Values {
				result[name] = append(result[name], strings.TrimSpace(v.Value))
			}
		}
	}
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		result["nameid"] = []string{assertion.Subject.NameID.Value}
	}
	return result
}

func attributeLookup(attrs map[string][]string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	values := attrs[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func attributeValues(attrs map[string][]string, key string) []string {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return attrs[key]
}
