package cassandra

import (
	"context"
	"errors"
	"testing"

	"github.com/gocql/gocql"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register Cassandra plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("Cassandra must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support Cassandra")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support Cassandra")
	}
}

func TestParseOptionsDefaultsToNoAuth(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"hosts": "127.0.0.1"}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "" || opts.Password != "" || opts.Port != defaultPort || opts.Consistency != gocql.LocalQuorum {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
}

func TestParseOptionsUsesPasswordCredentialAndTLSCredential(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"hosts":                   "db1, db2",
		"auth":                    authCredential,
		plugin.CredentialIdentity: "cassandra",
		plugin.CredentialSecret:   "secret",
		"tls_mode":                "require",
		"_client_cert_id_secret":  "pem-material",
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "cassandra" || opts.Password != "secret" || opts.ClientCertificate != "pem-material" || len(opts.Hosts) != 2 {
		t.Fatalf("unexpected credential material: %+v", opts)
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	_, err := executeQueryRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, sqldb.QueryRequest{Query: "insert into events (id) values (1)"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, sqldb.QueryRequest{Query: "truncate events"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != models.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestCQLDDLValidation(t *testing.T) {
	cols, err := parseColumns([]any{
		map[string]any{"name": "id", "type": "uuid"},
		map[string]any{"name": "payload", "type": "frozen<map<text, text>>"},
	})
	if err != nil {
		t.Fatalf("valid columns rejected: %v", err)
	}
	if len(cols) != 2 || cols[0] != `"id" uuid` {
		t.Fatalf("unexpected columns: %#v", cols)
	}
	if _, err := parseColumns([]any{map[string]any{"name": "bad-name", "type": "text"}}); err == nil {
		t.Fatal("invalid identifier accepted")
	}
	if safePrimaryKey("id); DROP TABLE users") {
		t.Fatal("unsafe primary key accepted")
	}
}
