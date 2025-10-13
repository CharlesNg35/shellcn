package providers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	saml "github.com/crewjam/saml"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestParsePrivateKeyAndCertificateChain(t *testing.T) {
	keyPEM, certPEM := generateKeyAndCertPEM(t)

	key, err := parsePrivateKey(keyPEM)
	if err != nil {
		t.Fatalf("parsePrivateKey returned error: %v", err)
	}
	if key.N.BitLen() == 0 {
		t.Fatal("expected RSA modulus to be set")
	}

	cert, intermediates, err := parseCertificateChain(certPEM)
	if err != nil {
		t.Fatalf("parseCertificateChain returned error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected primary certificate")
	}
	if len(intermediates) != 0 {
		t.Fatalf("expected no intermediates, got %d", len(intermediates))
	}
}

func TestParsePrivateKeyRejectsInvalid(t *testing.T) {
	if _, err := parsePrivateKey("not pem"); err == nil {
		t.Fatal("expected error for invalid pem")
	}
}

func TestEnsureTrailingPath(t *testing.T) {
	if got := ensureTrailingPath("/saml", "/metadata"); got != "/saml/metadata" {
		t.Fatalf("unexpected path: %s", got)
	}
	if got := ensureTrailingPath("/saml/", "/metadata"); got != "/saml/metadata" {
		t.Fatalf("unexpected path with slash: %s", got)
	}
	if got := ensureTrailingPath("/saml/metadata", "/metadata"); got != "/saml/metadata" {
		t.Fatalf("unexpected path when already suffixed: %s", got)
	}
}

func TestNormalizeAttributeMap(t *testing.T) {
	input := map[string]string{
		" Email ":      " mail ",
		"DISPLAY_NAME": " displayName ",
	}
	out := normalizeAttributeMap(input)
	if out["email"] != "mail" || out["display_name"] != "displayName" {
		t.Fatalf("unexpected normalized map: %#v", out)
	}
}

func TestCollectAttributes(t *testing.T) {
	assertion := &saml.Assertion{
		AttributeStatements: []saml.AttributeStatement{{
			Attributes: []saml.Attribute{
				{Name: "email", Values: []saml.AttributeValue{{Value: "user@example.com"}}},
				{Name: "groups", Values: []saml.AttributeValue{{Value: "admins"}, {Value: "devs"}}},
			},
		}},
		Subject: &saml.Subject{
			NameID: &saml.NameID{Value: "user-id"},
		},
	}

	attrs := collectAttributes(assertion)
	if attrs["email"][0] != "user@example.com" {
		t.Fatalf("email mismatch: %v", attrs["email"])
	}
	if len(attrs["groups"]) != 2 {
		t.Fatalf("expected 2 groups, got %v", attrs["groups"])
	}
	if attrs["nameid"][0] != "user-id" {
		t.Fatalf("expected nameid to be populated: %v", attrs["nameid"])
	}
}

func TestPopulateIDPMetadataFromURL(t *testing.T) {
	keyPEM, certPEM := generateKeyAndCertPEM(t)
	cert, _, err := parseCertificateChain(certPEM)
	if err != nil {
		t.Fatalf("parseCertificateChain returned error: %v", err)
	}

	metadataDoc := fmt.Sprintf(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com/metadata">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data>
          <X509Certificate>%s</X509Certificate>
        </X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, base64.StdEncoding.EncodeToString(cert.Raw))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(metadataDoc))
	}))
	t.Cleanup(server.Close)

	sp := &saml.ServiceProvider{
		Certificate: cert,
	}

	cfg := models.SAMLConfig{
		EntityID:    "https://sp.example.com/metadata",
		ACSURL:      "https://sp.example.com/acs",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
		MetadataURL: server.URL,
	}

	if err := populateIDPMetadata(server.Client(), cfg, sp, time.Second); err != nil {
		t.Fatalf("populateIDPMetadata returned error: %v", err)
	}
	if sp.IDPMetadata == nil {
		t.Fatal("expected IDP metadata to be populated")
	}
}

func TestPopulateIDPMetadataFromSSOConfig(t *testing.T) {
	keyPEM, certPEM := generateKeyAndCertPEM(t)
	sp := &saml.ServiceProvider{}
	cfg := models.SAMLConfig{
		EntityID:    "https://sp.example.com/metadata",
		ACSURL:      "https://sp.example.com/acs",
		Certificate: certPEM,
		PrivateKey:  keyPEM,
		SSOURL:      "https://idp.example.com/sso",
	}

	if err := populateIDPMetadata(nil, cfg, sp, time.Second); err != nil {
		t.Fatalf("populateIDPMetadata returned error: %v", err)
	}
	if sp.IDPMetadata == nil {
		t.Fatal("expected IDP metadata to be populated via SSO URL")
	}
}

func TestAttributeLookupAndValues(t *testing.T) {
	attrs := map[string][]string{
		"email": {"user@example.com"},
		"roles": {"admin", "dev"},
	}

	if got := attributeLookup(attrs, " email "); got != "user@example.com" {
		t.Fatalf("attributeLookup mismatch, got %q", got)
	}
	if vals := attributeValues(attrs, "roles"); len(vals) != 2 {
		t.Fatalf("expected role values, got %v", vals)
	}
	if vals := attributeValues(attrs, "missing"); vals != nil {
		t.Fatalf("expected nil for missing attribute, got %v", vals)
	}
}

func generateKeyAndCertPEM(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test SP",
			Organization: []string{"ShellCN"},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})

	return string(keyPEM), string(certPEM)
}
