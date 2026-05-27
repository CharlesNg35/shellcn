package plugins

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestSearchPluginsValidateAndRegister(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"elasticsearch", "opensearch", "meilisearch", "typesense", "solr"} {
		proj, ok := reg.Projection(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		if proj.Category.Key != plugin.CategorySearch {
			t.Fatalf("%s category: got %q want %q", name, proj.Category.Key, plugin.CategorySearch)
		}
		if proj.Layout != plugin.LayoutSidebarTree {
			t.Fatalf("%s should use sidebar tree layout, got %q", name, proj.Layout)
		}
		if len(proj.Resources) == 0 || len(proj.Actions) == 0 {
			t.Fatalf("%s should expose resources and actions", name)
		}
		if len(proj.SupportedTransports) != 1 || proj.SupportedTransports[0] != plugin.TransportDirect {
			t.Fatalf("%s should be direct transport only: %+v", name, proj.SupportedTransports)
		}
	}
}

func TestSearchCredentialCompatibility(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"elasticsearch", "opensearch"} {
		for _, kind := range []plugin.CredentialKind{plugin.CredentialBasicAuth, plugin.CredentialAPIToken, plugin.CredentialBearerToken} {
			if !reg.CredentialKindSupportsProtocol(kind, name) {
				t.Fatalf("%s credential should support %s", kind, name)
			}
		}
		if reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, name) {
			t.Fatalf("database password credentials should not support %s", name)
		}
	}
	for _, name := range []string{"meilisearch", "typesense"} {
		if !reg.CredentialKindSupportsProtocol(plugin.CredentialAPIToken, name) {
			t.Fatalf("api token credential should support %s", name)
		}
		for _, kind := range []plugin.CredentialKind{plugin.CredentialBasicAuth, plugin.CredentialBearerToken, plugin.CredentialDBPassword} {
			if reg.CredentialKindSupportsProtocol(kind, name) {
				t.Fatalf("%s should not advertise %s credentials", name, kind)
			}
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialBasicAuth, plugin.CredentialBearerToken} {
		if !reg.CredentialKindSupportsProtocol(kind, "solr") {
			t.Fatalf("%s credential should support solr", kind)
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialAPIToken, plugin.CredentialDBPassword} {
		if reg.CredentialKindSupportsProtocol(kind, "solr") {
			t.Fatalf("solr should not advertise %s credentials", kind)
		}
	}
}

func TestSearchSchemasAreProtocolSpecific(t *testing.T) {
	reg := plugin.NewRegistry()
	Register(reg)

	for _, name := range []string{"elasticsearch", "opensearch"} {
		manifest, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		fields := fieldMap(manifest.Config)
		for _, key := range []string{"endpoint", "auth", "api_key", "bearer_token", "credential_id", "tls_mode", "read_only", "page_limit"} {
			if !fields[key] {
				t.Fatalf("%s should expose search API field %q", name, key)
			}
		}
		for _, key := range []string{"confirm_writes", "host", "database", "brokers", "urls", "management_url", "keyspace"} {
			if fields[key] {
				t.Fatalf("%s should not expose non-search field %q", name, key)
			}
		}
	}
	for _, name := range []string{"meilisearch", "typesense"} {
		manifest, ok := reg.Manifest(name)
		if !ok {
			t.Fatalf("plugin %q was not registered", name)
		}
		fields := fieldMap(manifest.Config)
		for _, key := range []string{"endpoint", "auth", "api_key", "credential_id", "tls_mode", "read_only", "page_limit"} {
			if !fields[key] {
				t.Fatalf("%s should expose search API field %q", name, key)
			}
		}
		for _, key := range []string{"confirm_writes", "username", "password", "bearer_token", "database", "brokers", "urls", "management_url", "keyspace"} {
			if fields[key] {
				t.Fatalf("%s should not expose unrelated field %q", name, key)
			}
		}
	}
	manifest, ok := reg.Manifest("solr")
	if !ok {
		t.Fatalf("plugin %q was not registered", "solr")
	}
	fields := fieldMap(manifest.Config)
	for _, key := range []string{"endpoint", "auth", "username", "password", "bearer_token", "credential_id", "tls_mode", "read_only", "page_limit"} {
		if !fields[key] {
			t.Fatalf("solr should expose search API field %q", key)
		}
	}
	for _, key := range []string{"confirm_writes", "api_key", "database", "brokers", "urls", "management_url", "keyspace"} {
		if fields[key] {
			t.Fatalf("solr should not expose unrelated field %q", key)
		}
	}
}
