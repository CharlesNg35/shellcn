package mssql

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register MSSQL plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("MSSQL must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support MSSQL")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support MSSQL transport TLS")
	}
}

func TestParseOptionsUsesTLSClientCertificateCredential(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"host":                   "sql.local",
		"database":               "master",
		"username":               "sa",
		"password":               "secret",
		"encrypt":                "require",
		"_client_cert_id_secret": "pem-material",
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.ClientCertificate != "pem-material" || opts.Username != "sa" || opts.Password != "secret" {
		t.Fatalf("unexpected credential material: %+v", opts)
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	_, err := executeQueryRequest(context.Background(), &Session{opts: optionsData{ReadOnly: true}}, "master", sqldb.QueryRequest{Query: "delete from dbo.accounts"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: optionsData{RequireConfirm: true}}, "master", sqldb.QueryRequest{Query: "drop table dbo.accounts"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != models.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestMSSQLDDLColumnValidation(t *testing.T) {
	col, err := ddlColumn(sqldb.ColumnSpec{Name: "id", Type: "bigint identity(1,1)", Primary: true})
	if err != nil {
		t.Fatalf("valid column rejected: %v", err)
	}
	if col != "[id] bigint identity(1,1) NOT NULL PRIMARY KEY" {
		t.Fatalf("unexpected column: %q", col)
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "bad:name", Type: "nvarchar(255)"}); err == nil {
		t.Fatal("invalid identifier accepted")
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "name", Type: "nvarchar(255); drop table users"}); err == nil {
		t.Fatal("unsafe type accepted")
	}
}

func TestObjectIDRoundTrip(t *testing.T) {
	id := objectID("app", "dbo", "people")
	database, schema, name, err := parseObjectID(id)
	if err != nil {
		t.Fatalf("parse object id: %v", err)
	}
	if database != "app" || schema != "dbo" || name != "people" {
		t.Fatalf("unexpected identity: %s %s %s", database, schema, name)
	}
	if _, _, _, err := parseObjectID("app:bad:name:extra"); err == nil {
		t.Fatal("accepted ambiguous object id")
	}
}

func TestRedactRowsMasksConfiguredColumns(t *testing.T) {
	rows := []row{{"id": int64(1), "access_token": "plain", "name": "alice"}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["access_token"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
}

func TestTableDataGridIsEditable(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	var data plugin.Tab
	for _, res := range m.Resources {
		if res.Kind != "table" {
			continue
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Key == "data" {
				data = tab
			}
		}
	}
	tc, ok := data.Config.(plugin.TableConfig)
	if data.Key == "" || !ok || !tc.Editable {
		t.Fatalf("table Data tab must be an editable grid: %#v", data.Config)
	}
	for key, ds := range map[string]*plugin.DataSource{"insert": tc.Insert, "update": tc.Update, "delete": tc.Delete} {
		if ds == nil {
			t.Fatalf("Data tab missing %q mutation source", key)
		}
		if !routeIDs[ds.RouteID] {
			t.Fatalf("Data tab %q points at missing route %q", key, ds.RouteID)
		}
	}
}
