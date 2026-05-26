package plugins

import (
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestDatabaseCredentialSelectorsExposeOnlyAppropriateKinds(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"postgresql", "mysql", "redis", "mongodb", "cockroachdb", "clickhouse"} {
		m, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		field, ok := credentialField(m.Config, "credential_id")
		if !ok {
			t.Fatalf("%s should expose credential_id", name)
		}
		if !credentialKindsContain(field.Credential.Kinds, plugin.CredentialDBPassword) {
			t.Fatalf("%s auth credential selector should support database password: %+v", name, field.Credential.Kinds)
		}
		if credentialKindsContain(field.Credential.Kinds, plugin.CredentialTLSClientCert) {
			t.Fatalf("%s stored password selector should not advertise TLS client certificates: %+v", name, field.Credential.Kinds)
		}
	}

	for _, name := range []string{"postgresql", "mongodb", "cockroachdb", "clickhouse"} {
		m, _ := reg.Manifest(name)
		field, ok := credentialField(m.Config, "auth_client_cert_id")
		if !ok {
			t.Fatalf("%s should expose auth_client_cert_id for certificate authentication", name)
		}
		if !credentialKindsContain(field.Credential.Kinds, plugin.CredentialTLSClientCert) || credentialKindsContain(field.Credential.Kinds, plugin.CredentialDBPassword) {
			t.Fatalf("%s certificate auth selector should only advertise TLS client certificates: %+v", name, field.Credential.Kinds)
		}
	}

	for _, name := range []string{"mysql", "redis"} {
		m, _ := reg.Manifest(name)
		if _, ok := credentialField(m.Config, "auth_client_cert_id"); ok {
			t.Fatalf("%s should not expose certificate authentication", name)
		}
		tlsField, ok := credentialField(m.Config, "client_cert_id")
		if !ok {
			t.Fatalf("%s should expose client_cert_id in TLS settings", name)
		}
		if !credentialKindsContain(tlsField.Credential.Kinds, plugin.CredentialTLSClientCert) {
			t.Fatalf("%s TLS client certificate field should support TLS client certificates: %+v", name, tlsField.Credential.Kinds)
		}
	}
}

func credentialField(schema plugin.Schema, key string) (plugin.Field, bool) {
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key && field.Type == plugin.FieldCredentialRef && field.Credential != nil {
				return field, true
			}
		}
	}
	return plugin.Field{}, false
}

func credentialKindsContain(kinds []plugin.CredentialKind, want plugin.CredentialKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
