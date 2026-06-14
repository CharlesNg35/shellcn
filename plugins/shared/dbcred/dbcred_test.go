package dbcred

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestApplyPasswordCredentialIgnoresCredentialKindRouting(t *testing.T) {
	got := ApplyPasswordCredential(plugin.ConnectConfig{Credentials: plugin.NewResolvedCredentials(plugin.CredentialBinding{
		Field: plugin.CredentialIDField,
		Credential: plugin.ResolvedCredential{Kind: plugin.CredentialTLSClientCert, Values: map[string]string{
			"username": "default",
			"password": "redis-password",
		}},
	})}, "", "")
	if got.Username != "default" || got.Password != "redis-password" || got.ClientCertificate != "" || got.TLSMode != "" {
		t.Fatalf("unexpected password-only material: %+v", got)
	}
}

func TestApplyClientCertificateCredentialUsesFieldSpecificSecret(t *testing.T) {
	got := ApplyClientCertificateCredential(plugin.ConnectConfig{Credentials: plugin.NewResolvedCredentials(plugin.CredentialBinding{
		Field: "auth_client_cert_id",
		Credential: plugin.ResolvedCredential{Kind: plugin.CredentialTLSClientCert, Values: map[string]string{
			"subject":     "cert-user",
			"certificate": "cert-pem",
			"private_key": "key-pem",
		}},
	})}, "auth_client_cert_id", "", "disable", "")
	if got.Username != "cert-user" || got.Password != "" || got.ClientCertificate != "cert-pem\nkey-pem" || !got.UsedTLSClientCredential {
		t.Fatalf("unexpected client certificate material: %+v", got)
	}
	if got.TLSMode != "require" {
		t.Fatalf("client certificate should enable TLS, got %q", got.TLSMode)
	}
}
