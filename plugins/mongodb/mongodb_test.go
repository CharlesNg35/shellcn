package mongodb

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register MongoDB plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("MongoDB must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support MongoDB")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
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
	want := []any{1, -1}
	if len(values) != len(want) {
		t.Fatalf("keys select options = %#v, want %#v", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("keys select option %d = %#v (%T), want %#v", i, values[i], values[i], want[i])
		}
	}
}
