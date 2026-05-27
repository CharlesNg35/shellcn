package plugin

import "testing"

func TestCredentialResolutionKeys(t *testing.T) {
	cfg := ConnectConfig{Config: map[string]any{
		CredentialSecretKey(CredentialField):       "secret",
		CredentialIdentityKey(CredentialField):     "identity",
		CredentialResolvedKindKey(CredentialField): string(CredentialAPIToken),
		CredentialSecretKey("api_credential"):      "field-secret",
		CredentialIdentityKey("api_credential"):    "field-identity",
		CredentialResolvedKindKey("api_credential"): string(
			CredentialBearerToken,
		),
	}}

	if got := cfg.CredentialSecretFor(CredentialField); got != "secret" {
		t.Fatalf("default secret = %q", got)
	}
	if got := cfg.CredentialIdentityFor(CredentialField); got != "identity" {
		t.Fatalf("default identity = %q", got)
	}
	if got := cfg.CredentialKindFor(CredentialField); got != CredentialAPIToken {
		t.Fatalf("default kind = %q", got)
	}
	if got := cfg.CredentialSecretFor("api_credential"); got != "field-secret" {
		t.Fatalf("field secret = %q", got)
	}
	if got := cfg.CredentialIdentityFor("api_credential"); got != "field-identity" {
		t.Fatalf("field identity = %q", got)
	}
	if got := cfg.CredentialKindFor("api_credential"); got != CredentialBearerToken {
		t.Fatalf("field kind = %q", got)
	}
}
