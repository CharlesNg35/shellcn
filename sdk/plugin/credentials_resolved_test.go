package plugin

import "testing"

func TestConnectConfigCredentialHelpersUseResolvedCredentials(t *testing.T) {
	cfg := ConnectConfig{Credentials: NewResolvedCredentials(
		CredentialBinding{Field: CredentialRefField, Credential: ResolvedCredential{
			ID:     "cred-default",
			Kind:   CredentialKindAPIToken,
			Values: map[string]string{"token": "secret", "subject": "identity"},
		}},
		CredentialBinding{Field: "api_credential", Credential: ResolvedCredential{
			ID:     "cred-field",
			Kind:   CredentialKindBearerToken,
			Values: map[string]string{"token": "field-secret", "subject": "field-identity"},
		}},
	)}
	if got := cfg.CredentialValueFor(CredentialRefField, "token"); got != "secret" {
		t.Fatalf("default credential token = %q", got)
	}
	if got := cfg.CredentialValueFor(CredentialRefField, "subject"); got != "identity" {
		t.Fatalf("default credential subject = %q", got)
	}
	if got := cfg.CredentialKindFor(CredentialRefField); got != CredentialKindAPIToken {
		t.Fatalf("default credential kind = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "token"); got != "field-secret" {
		t.Fatalf("field credential token = %q", got)
	}
	if got := cfg.CredentialValueFor("api_credential", "subject"); got != "field-identity" {
		t.Fatalf("field credential subject = %q", got)
	}
	if got := cfg.CredentialKindFor("api_credential"); got != CredentialKindBearerToken {
		t.Fatalf("field credential kind = %q", got)
	}

	cred, err := cfg.RequiredCredentialFor("api_credential", CredentialKindBearerToken)
	if err != nil {
		t.Fatalf("required credential: %v", err)
	}
	if cred.ID != "cred-field" {
		t.Fatalf("credential id = %q", cred.ID)
	}
}
