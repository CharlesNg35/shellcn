package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestObservabilityPluginsValidateAndRegister(t *testing.T) {
	for _, name := range []string{"server_monitor"} {
		proj := testProjection(t, name)
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
	manifest := testManifest(t, "server_monitor")

	for _, kind := range []plugin.CredentialKind{plugin.CredentialKindBasicAuth, plugin.CredentialKindBearerToken, plugin.CredentialKindAPIToken, plugin.CredentialKindDBPassword} {
		if plugintest.CredentialKindSupported(manifest.Config, kind) {
			t.Fatalf("server_monitor should not advertise %s credentials", kind)
		}
	}
}

func TestObservabilitySchemasAreProtocolSpecific(t *testing.T) {
	manifest := testManifest(t, "server_monitor")
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
