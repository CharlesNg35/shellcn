package typesense

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()
	if err := plugin.Validate(m, p.Routes()); err != nil {
		t.Fatalf("manifest should validate: %v", err)
	}
	if m.Category != plugin.CategorySearch {
		t.Fatalf("category: got %q want %q", m.Category, plugin.CategorySearch)
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("typesense should be direct-only: %+v", m.SupportedTransports)
	}
	fields := fieldMap(m.Config)
	for _, key := range []string{"endpoint", "auth", "api_key", "credential_id", "tls_mode", "read_only", "page_limit"} {
		if !fields[key] {
			t.Fatalf("missing field %q", key)
		}
	}
	for _, key := range []string{"username", "password", "bearer_token"} {
		if fields[key] {
			t.Fatalf("unexpected field %q", key)
		}
	}
}

func TestSynonymAndCurationRoutesAreGlobal(t *testing.T) {
	routes := routeMap(New().Routes())
	for id, path := range map[string]string{
		rid("synonyms.list"):   "/synonym_sets",
		rid("synonym.upsert"):  "/synonym_sets/{synonym}",
		rid("synonym.delete"):  "/synonym_sets/{synonym}",
		rid("overrides.list"):  "/curation_sets",
		rid("override.upsert"): "/curation_sets/{override}",
		rid("override.delete"): "/curation_sets/{override}",
	} {
		route, ok := routes[id]
		if !ok {
			t.Fatalf("missing route %q", id)
		}
		if route.Path != path {
			t.Fatalf("%s path: got %q want %q", id, route.Path, path)
		}
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	fields := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			fields[field.Key] = true
		}
	}
	return fields
}
