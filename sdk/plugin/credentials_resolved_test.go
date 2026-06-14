package plugin

import "testing"

func TestCredentialResolutionKeys(t *testing.T) {
	cfg := ConnectConfig{Config: map[string]any{
		CredentialValuesKey(CredentialIDField):       map[string]string{"token": "secret", "subject": "identity"},
		CredentialResolvedKindKey(CredentialIDField): string(CredentialAPIToken),
		CredentialValuesKey("api_credential"):        map[string]any{"token": "field-secret", "subject": "field-identity"},
		CredentialResolvedKindKey("api_credential"): string(
			CredentialBearerToken,
		),
	}}

	if got := cfg.CredentialValueFor(CredentialIDField, "token"); got != "secret" {
		t.Fatalf("default token = %q", got)
	}
	if got := cfg.CredentialValueFor(CredentialIDField, "subject"); got != "identity" {
		t.Fatalf("default subject = %q", got)
	}
	if got := cfg.CredentialKindFor(CredentialIDField); got != CredentialAPIToken {
		t.Fatalf("default kind = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "token"); got != "field-secret" {
		t.Fatalf("field token = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "subject"); got != "field-identity" {
		t.Fatalf("field subject = %q", got)
	}
	if got := cfg.CredentialKindFor("api_credential"); got != CredentialBearerToken {
		t.Fatalf("field kind = %q", got)
	}
}
