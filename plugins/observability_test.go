package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestObservabilityPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"prometheus"} {
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
}
