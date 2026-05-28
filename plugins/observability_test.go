package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestObservabilityPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"prometheus", "influxdb"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategoryObservability {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategoryObservability)
		}
		if proj.Layout != plugin.LayoutSidebarTree {
			t.Fatalf("%s should use sidebar tree layout, got %q", name, proj.Layout)
		}
		if len(proj.Resources) == 0 || len(proj.Actions) == 0 || len(proj.Streams) == 0 {
			t.Fatalf("%s should expose resources, actions, and streams", name)
		}
		if len(proj.SupportedTransports) != 1 || proj.SupportedTransports[0] != plugin.TransportDirect {
			t.Fatalf("%s should be direct transport only: %+v", name, proj.SupportedTransports)
		}
	}
}

func TestObservabilityCredentialCompatibility(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, kind := range []plugin.CredentialKind{plugin.CredentialBasicAuth, plugin.CredentialBearerToken} {
		if !reg.CredentialKindSupportsProtocol(kind, "prometheus") {
			t.Fatalf("%s credential should support prometheus", kind)
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialAPIToken, plugin.CredentialDBPassword} {
		if reg.CredentialKindSupportsProtocol(kind, "prometheus") {
			t.Fatalf("prometheus should not advertise %s credentials", kind)
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialAPIToken, plugin.CredentialBearerToken, plugin.CredentialBasicAuth} {
		if !reg.CredentialKindSupportsProtocol(kind, "influxdb") {
			t.Fatalf("%s credential should support influxdb", kind)
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialDBPassword, plugin.CredentialTLSClientCert} {
		if reg.CredentialKindSupportsProtocol(kind, "influxdb") {
			t.Fatalf("influxdb should not advertise %s credentials", kind)
		}
	}
}

func TestObservabilitySchemasAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	manifest, ok := reg.Manifest("prometheus")
	if !ok {
		t.Fatal("prometheus plugin was not registered")
	}
	fields := fieldMap(manifest.Config)
	for _, key := range []string{"endpoint", "auth", "username", "password", "bearer_token", "credential_id", "tls_mode", "timeout", "poll_interval", "page_limit", "admin_api", "lifecycle_api"} {
		if !fields[key] {
			t.Fatalf("prometheus should expose field %q", key)
		}
	}
	for _, key := range []string{"api_key", "database", "keyspace", "brokers", "management_url", "read_only", "confirm_writes"} {
		if fields[key] {
			t.Fatalf("prometheus should not expose unrelated field %q", key)
		}
	}

	manifest, ok = reg.Manifest("influxdb")
	if !ok {
		t.Fatal("influxdb plugin was not registered")
	}
	fields = fieldMap(manifest.Config)
	for _, key := range []string{
		"api_mode", "endpoint", "org", "database", "auth_v3", "auth_v2", "auth_v1",
		"api_token_v3", "token_credential_v3_id", "username_v3", "password_v3", "basic_credential_v3_id",
		"api_token_v2", "token_credential_v2_id", "username_v1", "password_v1", "basic_credential_v1_id",
		"tls_mode", "ca_certificate", "query_language_v3", "lookback", "timeout", "page_limit", "read_only", "confirm_writes",
	} {
		if !fields[key] {
			t.Fatalf("influxdb should expose field %q", key)
		}
	}
	for _, key := range []string{"auth", "api_token", "token_credential_id", "basic_credential_id", "bearer_token", "credential_id", "poll_interval", "admin_api", "lifecycle_api", "keyspace", "brokers", "management_url"} {
		if fields[key] {
			t.Fatalf("influxdb should not expose unrelated or ambiguous field %q", key)
		}
	}
}
