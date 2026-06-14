package plugin

import "testing"

func TestConnectConfigCredentialHelpersUseResolvedCredentials(t *testing.T) {
	cfg := ConnectConfig{Credentials: NewResolvedCredentials(
		CredentialBinding{Field: CredentialIDField, Credential: ResolvedCredential{
			ID:     "cred-default",
			Kind:   CredentialAPIToken,
			Values: map[string]string{"token": "secret", "subject": "identity"},
		}},
		CredentialBinding{Field: "api_credential", Credential: ResolvedCredential{
			ID:     "cred-field",
			Kind:   CredentialBearerToken,
			Values: map[string]string{"token": "field-secret", "subject": "field-identity"},
		}},
	)}
	if got := cfg.CredentialValueFor(CredentialIDField, "token"); got != "secret" {
		t.Fatalf("default credential token = %q", got)
	}
	if got := cfg.CredentialValueFor(CredentialIDField, "subject"); got != "identity" {
		t.Fatalf("default credential subject = %q", got)
	}
	if got := cfg.CredentialKindFor(CredentialIDField); got != CredentialAPIToken {
		t.Fatalf("default credential kind = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "token"); got != "field-secret" {
		t.Fatalf("field credential token = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "subject"); got != "field-identity" {
		t.Fatalf("field credential subject = %q", got)
	}
	if got := cfg.CredentialKindFor("api_credential"); got != CredentialBearerToken {
		t.Fatalf("field credential kind = %q", got)
	}

	cred, err := cfg.RequiredCredentialFor("api_credential", CredentialBearerToken)
	if err != nil {
		t.Fatalf("required credential: %v", err)
	}
	if cred.ID != "cred-field" {
		t.Fatalf("credential id = %q", cred.ID)
	}
}
