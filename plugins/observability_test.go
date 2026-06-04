package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestObservabilityPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"server_monitor"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategoryObservability {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategoryObservability)
		}
		if proj.Layout != plugin.LayoutTabs {
			t.Fatalf("%s should use tabs layout, got %q", name, proj.Layout)
		}
		if len(proj.Tabs) == 0 || len(proj.Streams) == 0 {
			t.Fatalf("%s should expose tabs and streams", name)
		}
		if len(proj.SupportedTransports) != 2 || proj.SupportedTransports[0] != plugin.TransportDirect || proj.SupportedTransports[1] != plugin.TransportAgent {
			t.Fatalf("%s should support direct and agent transports: %+v", name, proj.SupportedTransports)
		}
	}
}

func TestObservabilityCredentialCompatibility(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, kind := range []plugin.CredentialKind{plugin.CredentialBasicAuth, plugin.CredentialBearerToken, plugin.CredentialAPIToken, plugin.CredentialDBPassword} {
		if reg.CredentialKindSupportsProtocol(kind, "server_monitor") {
			t.Fatalf("server_monitor should not advertise %s credentials", kind)
		}
	}
}

func TestObservabilitySchemasAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	manifest, ok := reg.Manifest("server_monitor")
	if !ok {
		t.Fatal("server_monitor plugin was not registered")
	}
	fields := fieldMap(manifest.Config)
	for _, key := range []string{"metrics_interval_seconds", "process_limit", "connection_limit"} {
		if !fields[key] {
			t.Fatalf("server_monitor should expose field %q", key)
		}
	}
	for _, key := range []string{"endpoint", "auth", "username", "password", "bearer_token", "credential_id", "tls_mode", "timeout", "poll_interval", "page_limit", "admin_api", "lifecycle_api"} {
		if fields[key] {
			t.Fatalf("server_monitor should not expose unrelated field %q", key)
		}
	}
}
