package neo4j

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersDirectOnlyAndCredentialKinds(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register Neo4j plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("Neo4j must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support Neo4j")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialBearerToken, protocolName) {
		t.Fatal("bearer token credential should support Neo4j")
	}
	if reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("Neo4j should not advertise TLS client certificate credentials")
	}
}

func TestConfigSchemaHasOnlyNeo4jFields(t *testing.T) {
	fields := fieldMap(configSchema())
	for _, key := range []string{"scheme", "host", "port", "database", "auth", "username", "credential_id", "password", "realm", "bearer_token", "bearer_credential_id", "ca_certificate", "read_only", "require_write_confirmation", "query_timeout", "connect_timeout", "retry_time", "pool_size", "fetch_size", "page_limit", "redact_properties"} {
		if !fields[key] {
			t.Fatalf("schema should expose %q", key)
		}
	}
	for _, key := range []string{"tls_mode", "client_cert_id", "auth_client_cert_id", "access_key_id", "secret_access_key", "endpoint", "keyspace"} {
		if fields[key] {
			t.Fatalf("schema should not expose unrelated field %q", key)
		}
	}
}

func TestConfigSchemaVisibleValuesAreAuthSpecific(t *testing.T) {
	schema := configSchema()
	tests := []struct {
		name   string
		values map[string]any
		want   []string
		reject []string
	}{
		{name: "password", values: map[string]any{"auth": authPassword, "scheme": "bolt"}, want: []string{"username", "password", "realm"}, reject: []string{"credential_id", "bearer_token", "bearer_credential_id", "ca_certificate"}},
		{name: "stored password", values: map[string]any{"auth": authCredential, "scheme": "bolt"}, want: []string{"credential_id"}, reject: []string{"username", "password", "realm", "bearer_token", "bearer_credential_id", "ca_certificate"}},
		{name: "bearer", values: map[string]any{"auth": authBearer, "scheme": "neo4j+s"}, want: []string{"bearer_token", "ca_certificate"}, reject: []string{"username", "password", "credential_id", "bearer_credential_id"}},
		{name: "stored bearer", values: map[string]any{"auth": authStoredBearer, "scheme": "bolt"}, want: []string{"bearer_credential_id"}, reject: []string{"username", "password", "credential_id", "bearer_token", "ca_certificate"}},
		{name: "none", values: map[string]any{"auth": authNone, "scheme": "bolt+ssc"}, want: []string{"auth", "scheme"}, reject: []string{"username", "password", "credential_id", "bearer_token", "bearer_credential_id", "ca_certificate"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible := visibleFields(schema, tt.values)
			for _, key := range tt.want {
				if !visible[key] {
					t.Fatalf("visible values should include %q in %#v", key, visible)
				}
			}
			for _, key := range tt.reject {
				if visible[key] {
					t.Fatalf("visible values should not include %q in %#v", key, visible)
				}
			}
		})
	}
}

func TestCypherSafety(t *testing.T) {
	for _, query := range []string{"MATCH (n) RETURN n", "SHOW INDEXES", "EXPLAIN CREATE (n)", "RETURN 'CREATE (n)' AS text"} {
		if cypherNeedsReview(query) {
			t.Fatalf("query should be read-only: %s", query)
		}
	}
	for _, query := range []string{"CREATE (n)", "MATCH (n) SET n.name = 'Ada'", "MATCH (n) DETACH DELETE n", "CALL dbms.listConfig()"} {
		if !cypherNeedsReview(query) {
			t.Fatalf("query should require review: %s", query)
		}
	}
}

func TestCypherSafetyStopsBeforeNetwork(t *testing.T) {
	_, err := executeCypher(context.Background(), &Session{opts: options{ReadOnly: true}}, defaultDatabase, sqldb.QueryRequest{Query: "CREATE (n)"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeCypher(context.Background(), &Session{opts: options{RequireConfirm: true}}, defaultDatabase, sqldb.QueryRequest{Query: "MERGE (n:Person {id: 1})"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestResourceIDRoundTrip(t *testing.T) {
	id := mustEncodeID("node", "neo4j", "4:abc")
	kind, db, elementID, err := decodeID3(id)
	if err != nil {
		t.Fatalf("decode id: %v", err)
	}
	if kind != "node" || db != "neo4j" || elementID != "4:abc" {
		t.Fatalf("unexpected decoded id: %s %s %s", kind, db, elementID)
	}
}

func TestManifestReferencesResolve(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	actions := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actions[action.ID] = action
		if !routeIDs[action.RouteID] {
			t.Fatalf("action %q points at missing route %q", action.ID, action.RouteID)
		}
	}
	for _, group := range m.Tree {
		if !routeIDs[group.Source.RouteID] {
			t.Fatalf("tree group %q points at missing route %q", group.Key, group.Source.RouteID)
		}
	}
	for _, res := range m.Resources {
		if !routeIDs[res.List.RouteID] {
			t.Fatalf("resource %q list points at missing route %q", res.Kind, res.List.RouteID)
		}
		for _, id := range append(append([]string{}, res.ActionIDs...), append(res.ListActionIDs, res.RowActionIDs...)...) {
			if _, ok := actions[id]; !ok {
				t.Fatalf("resource %q references missing action %q", res.Kind, id)
			}
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Source != nil && !routeIDs[tab.Source.RouteID] {
				t.Fatalf("resource %q tab %q points at missing route %q", res.Kind, tab.Key, tab.Source.RouteID)
			}
		}
	}
}

func fieldMap(schema plugin.Schema) map[string]bool {
	out := map[string]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			out[field.Key] = true
		}
	}
	return out
}

func visibleFields(schema plugin.Schema, overrides map[string]any) map[string]bool {
	values := schema.Defaults()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if _, ok := values[field.Key]; !ok {
				values[field.Key] = ""
			}
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	visible := schema.VisibleValues(values, nil)
	out := map[string]bool{}
	for key := range visible {
		out[key] = true
	}
	return out
}
