package mongodb

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	p := New()
	m := p.Manifest()
	plugintest.ValidatePlugin(t, p)
	if m.Agent != nil {
		t.Fatal("MongoDB must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialDBPassword) {
		t.Fatal("database password credential should support MongoDB")
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialTLSClientCert) {
		t.Fatal("TLS client certificate credential should support MongoDB")
	}
}

func TestCommandSafetyStopsBeforeMongo(t *testing.T) {
	_, err := executeCommandRequest(context.Background(), &Session{opts: optionsData{ReadOnly: true}}, "test", sqldb.QueryRequest{Query: `{"drop": "users"}`})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeCommandRequest(context.Background(), &Session{opts: optionsData{RequireConfirm: true}}, "test", sqldb.QueryRequest{Query: `{"insert": "users", "documents": [{"name": "ada"}]}`})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestParseOptionsUsesTLSCredentialAsX509Auth(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"host":                          "mongo.local",
		"auth":                          authClientCert,
		"_auth_client_cert_id_identity": "CN=app",
		"_auth_client_cert_id_secret":   "pem-material",
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "CN=app" || opts.Password != "" || opts.ClientCertificate != "pem-material" || opts.TLSMode != "require" || opts.AuthMechanism != "MONGODB-X509" || opts.AuthSource != "$external" {
		t.Fatalf("unexpected credential material: %+v", opts)
	}
}

func TestDocumentIDRoundTrip(t *testing.T) {
	id, err := encodeDocumentID("app", "users", "user-1")
	if err != nil {
		t.Fatalf("encode document id: %v", err)
	}
	database, collection, filter, err := documentFilter(id)
	if err != nil {
		t.Fatalf("decode document id: %v", err)
	}
	if database != "app" || collection != "users" || filter[0].Value != "user-1" {
		t.Fatalf("unexpected identity: %s %s %#v", database, collection, filter)
	}
}

func TestParseExtJSON(t *testing.T) {
	doc, err := parseExtJSON(`{"_id":{"$oid":"64b64c2f9f1b2c3d4e5f6789"},"name":"ada"}`)
	if err != nil {
		t.Fatalf("parse extended JSON: %v", err)
	}
	if doc["name"] != "ada" {
		t.Fatalf("unexpected document: %#v", doc)
	}
}

func TestDisplayValueFormatsBSONIDs(t *testing.T) {
	if got := displayValue("_id", map[string]any{"$oid": "64b64c2f9f1b2c3d4e5f6789"}); got != "64b64c2f9f1b2c3d4e5f6789" {
		t.Fatalf("object id display: got %#v", got)
	}
	uuidBytes := []any{32, 90, 17, 95, 100, 227, 74, 220, 141, 56, 74, 239, 117, 197, 139, 249}
	if got := displayValue("id", uuidBytes); got != "205a115f-64e3-4adc-8d38-4aef75c58bf9" {
		t.Fatalf("uuid byte display: got %#v", got)
	}
	if got := displayValue("tags", uuidBytes); got != `[32,90,17,95,100,227,74,220,141,56,74,239,117,197,139,249]` {
		t.Fatalf("non-id array should stay JSON: got %#v", got)
	}
}

func TestIndexCreateKeysIsMapOfDirectionSelect(t *testing.T) {
	var schema *plugin.Schema
	for _, r := range New().Routes() {
		if r.ID == "mongodb.index.create" {
			schema = r.Input
		}
	}
	if schema == nil {
		t.Fatal("mongodb.index.create has no input schema")
	}
	var field *plugin.Field
	for _, g := range schema.Groups {
		for i := range g.Fields {
			if g.Fields[i].Key == "keys" {
				field = &g.Fields[i]
			}
		}
	}
	if field == nil {
		t.Fatal("no keys field")
	}
	if field.Type != plugin.FieldMap {
		t.Fatalf("keys is %q, want map", field.Type)
	}
	if field.Item == nil || field.Item.Type != plugin.FieldSelect {
		t.Fatalf("keys value item is not a select")
	}
	values := make([]any, 0, len(field.Item.Options))
	for _, o := range field.Item.Options {
		values = append(values, o.Value)
	}
	want := []any{1, -1, "text", "hashed", "2dsphere"}
	if len(values) != len(want) {
		t.Fatalf("keys select options = %#v, want %#v", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("keys select option %d = %#v (%T), want %#v", i, values[i], values[i], want[i])
		}
	}
}

func TestCollectionHasValidationTabAndIndexProperties(t *testing.T) {
	var collection plugin.ResourceType
	for _, res := range New().Manifest().Resources {
		if res.Kind == "collection" {
			collection = res
			break
		}
	}
	hasValidation := false
	for _, tab := range collection.Detail.Tabs {
		if tab.Key == "validation" {
			hasValidation = true
			if tab.Type != plugin.PanelObjectDetail || tab.Source == nil || tab.Source.RouteID != "mongodb.collection.validation" {
				t.Fatalf("validation tab is not backed by collection validation: %#v", tab)
			}
		}
	}
	if !hasValidation {
		t.Fatal("collection detail missing validation tab")
	}
	cols := map[string]bool{}
	for _, col := range indexColumns() {
		cols[col.Key] = true
	}
	for _, key := range []string{"type", "hidden", "ttl", "properties"} {
		if !cols[key] {
			t.Fatalf("index table missing %s column", key)
		}
	}
}

func TestDatabaseOverviewUsesGenericDashboard(t *testing.T) {
	m := New().Manifest()
	var overview plugin.Panel
	for _, res := range m.Resources {
		if res.Kind != "database" {
			continue
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Key == "overview" {
				overview = tab
			}
		}
	}
	cfg, ok := overview.Config.(plugin.DashboardConfig)
	if overview.Type != plugin.PanelDashboard || !ok {
		t.Fatalf("database overview should be a generic dashboard: %#v", overview)
	}
	cells := map[string]plugin.Panel{}
	for _, cell := range cfg.Cells {
		cells[cell.Key] = cell
	}
	if cells["summary"].Type != plugin.PanelObjectDetail || cells["summary"].Source == nil || cells["summary"].Source.RouteID != "mongodb.database.overview" {
		t.Fatalf("summary cell should render database overview details: %#v", cells["summary"])
	}
	if len(cells) != 1 {
		t.Fatalf("database overview should not duplicate the Collections tab: %#v", cells)
	}
}

func TestIndexCreateAdvancedOptionsAreStateSpecific(t *testing.T) {
	schema := routeInputSchema(t, "mongodb.index.create")
	for _, key := range []string{"hidden", "ttl", "partial"} {
		field := requireRouteField(t, schema, key)
		if field.Type != plugin.FieldToggle {
			t.Fatalf("%s should be a toggle: %#v", key, field)
		}
	}
	ttl := requireRouteField(t, schema, "expire_after_seconds")
	if ttl.Type != plugin.FieldNumber || ttl.VisibleWhen == nil {
		t.Fatalf("TTL seconds should be a state-specific number field: %#v", ttl)
	}
	partial := requireRouteField(t, schema, "partial_filter")
	if partial.Type != plugin.FieldJSON || partial.VisibleWhen == nil {
		t.Fatalf("partial filter should be a state-specific JSON field: %#v", partial)
	}
}

func TestMongoNameFieldsValidateClientSide(t *testing.T) {
	for _, tc := range []struct {
		routeID string
		field   string
		valid   map[string]any
		invalid map[string]any
	}{
		{
			routeID: "mongodb.database.create",
			field:   "name",
			valid:   map[string]any{"name": "app", "collection": "users"},
			invalid: map[string]any{"name": "$cmd", "collection": "users"},
		},
		{
			routeID: "mongodb.collection.create",
			field:   "name",
			valid:   map[string]any{"name": "users"},
			invalid: map[string]any{"name": "bad/name"},
		},
		{
			routeID: "mongodb.index.create",
			field:   "name",
			valid:   map[string]any{"keys": map[string]any{"email": 1}, "name": "email_1"},
			invalid: map[string]any{"keys": map[string]any{"email": 1}, "name": `bad\name`},
		},
	} {
		schema := routeInputSchema(t, tc.routeID)
		field := requireRouteField(t, schema, tc.field)
		if len(field.Validators) == 0 {
			t.Fatalf("%s.%s should declare a validator", tc.routeID, tc.field)
		}
		if err := schema.ValidateValues(tc.valid, nil); err != nil {
			t.Fatalf("%s valid values rejected: %v", tc.routeID, err)
		}
		if err := schema.ValidateValues(tc.invalid, nil); !errors.Is(err, plugin.ErrInvalidInput) {
			t.Fatalf("%s invalid values accepted: %v", tc.routeID, err)
		}
	}
}

func TestCollectionCreateCappedSizeOnlyVisibleWhenCapped(t *testing.T) {
	schema := routeInputSchema(t, "mongodb.collection.create")
	visible := schema.VisibleValues(map[string]any{"name": "events", "capped": false}, nil)
	if _, ok := visible["size"]; ok {
		t.Fatal("capped collection size should be hidden unless capped is enabled")
	}
	if err := schema.ValidateValues(map[string]any{"name": "events", "capped": false}, nil); err != nil {
		t.Fatalf("uncapped collection should not require size: %v", err)
	}
	if err := schema.ValidateValues(map[string]any{"name": "events", "capped": true}, nil); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("capped collection without size should be invalid, got %v", err)
	}
}

func TestDocumentDetailOpensReadOnlyViewFirst(t *testing.T) {
	var document plugin.ResourceType
	for _, res := range New().Manifest().Resources {
		if res.Kind == "document" {
			document = res
		}
	}
	if document.Detail.DefaultTab != "document" {
		t.Fatalf("document default tab = %q, want read-only document", document.Detail.DefaultTab)
	}
}

func TestDestructiveResourceActionsNavigateAwayFromDeletedDetails(t *testing.T) {
	actions := map[string]plugin.Action{}
	for _, a := range New().Manifest().Actions {
		actions[a.ID] = a
	}
	for _, id := range []string{"mongodb.collection.drop", "mongodb.document.delete"} {
		action := actions[id]
		if !action.Confirm {
			t.Fatalf("%s must require confirmation", id)
		}
		if action.OnSuccess == nil || action.OnSuccess.Navigate != plugin.NavigateList {
			t.Fatalf("%s should navigate back to the list after success: %#v", id, action.OnSuccess)
		}
	}
}

func routeInputSchema(t *testing.T, routeID string) *plugin.Schema {
	t.Helper()
	for _, r := range New().Routes() {
		if r.ID == routeID {
			if r.Input == nil {
				t.Fatalf("%s has no input schema", routeID)
			}
			return r.Input
		}
	}
	t.Fatalf("route %s not found", routeID)
	return nil
}

func requireRouteField(t *testing.T, schema *plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, g := range schema.Groups {
		for _, field := range g.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("schema missing %s", key)
	return plugin.Field{}
}
