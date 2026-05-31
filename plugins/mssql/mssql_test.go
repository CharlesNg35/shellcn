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

func TestConstraintClauseGeneration(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		columns any
		check   string
		ref     string
		refcols string
		want    string
	}{
		{name: "pk_app", kind: "PRIMARY KEY", columns: []any{"id"}, want: "CONSTRAINT [pk_app] PRIMARY KEY ([id])"},
		{name: "uq_email", kind: "UNIQUE", columns: "email", want: "CONSTRAINT [uq_email] UNIQUE ([email])"},
		{name: "uq_pair", kind: "unique", columns: []any{"a", "b"}, want: "CONSTRAINT [uq_pair] UNIQUE ([a], [b])"},
		{name: "ck_age", kind: "CHECK", check: "[age] >= 0", want: "CONSTRAINT [ck_age] CHECK ([age] >= 0)"},
		{name: "fk_person", kind: "FOREIGN KEY", columns: "person_id", ref: "dbo.people", refcols: "id", want: "CONSTRAINT [fk_person] FOREIGN KEY ([person_id]) REFERENCES [dbo].[people] ([id])"},
	}
	for _, tc := range cases {
		got, err := constraintClause(tc.name, tc.kind, tc.columns, tc.check, tc.ref, tc.refcols)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestConstraintClauseRejectsUnsafe(t *testing.T) {
	if _, err := constraintClause("ck", "CHECK", nil, "1=1; DROP TABLE x", "", ""); err == nil {
		t.Fatal("unsafe check expression accepted")
	}
	if _, err := constraintClause("pk", "PRIMARY KEY", nil, "", "", ""); err == nil {
		t.Fatal("primary key without columns accepted")
	}
	if _, err := constraintClause("fk", "FOREIGN KEY", "a", "", "bad.ref.table", "id"); err == nil {
		t.Fatal("over-qualified referenced table accepted")
	}
	if _, err := constraintClause("bad:name", "UNIQUE", "a", "", "", ""); err == nil {
		t.Fatal("invalid constraint identifier accepted")
	}
	if _, err := constraintClause("c", "GIBBERISH", "a", "", "", ""); err == nil {
		t.Fatal("unknown constraint type accepted")
	}
}

func TestRefTableClause(t *testing.T) {
	if got, err := refTableClause("people"); err != nil || got != "[people]" {
		t.Fatalf("bare table: got %q err=%v", got, err)
	}
	if got, err := refTableClause("dbo.people"); err != nil || got != "[dbo].[people]" {
		t.Fatalf("qualified table: got %q err=%v", got, err)
	}
	if _, err := refTableClause(""); err == nil {
		t.Fatal("empty referenced table accepted")
	}
}

func TestSingleIdentValue(t *testing.T) {
	if got, err := singleIdentValue("name"); err != nil || got != "name" {
		t.Fatalf("string ident: got %q err=%v", got, err)
	}
	if got, err := singleIdentValue([]any{"name"}); err != nil || got != "name" {
		t.Fatalf("array ident: got %q err=%v", got, err)
	}
	if _, err := singleIdentValue([]any{"a", "b"}); err == nil {
		t.Fatal("multi-element array accepted")
	}
	if _, err := singleIdentValue("bad:name"); err == nil {
		t.Fatal("invalid identifier accepted")
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
	var data plugin.Panel
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
