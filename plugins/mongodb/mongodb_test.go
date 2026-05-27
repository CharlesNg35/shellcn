package mongodb

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
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
