package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestManifestRegistersAndStaysDirectOnly(t *testing.T) {
	reg := plugin.NewRegistry()
	if err := reg.Register(New()); err != nil {
		t.Fatalf("register MySQL plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("MySQL must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support MySQL")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support MySQL")
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	_, err := executeQueryRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, sqldb.QueryRequest{Query: "delete from accounts"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, sqldb.QueryRequest{Query: "drop table accounts"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != models.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestMySQLDDLColumnValidation(t *testing.T) {
	col, err := ddlColumn(sqldb.ColumnSpec{Name: "id", Type: "bigint unsigned auto_increment", Primary: true})
	if err != nil {
		t.Fatalf("valid column rejected: %v", err)
	}
	if col != "`id` bigint unsigned auto_increment NOT NULL PRIMARY KEY" {
		t.Fatalf("unexpected column: %q", col)
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "bad-name", Type: "text"}); err == nil {
		t.Fatal("invalid identifier accepted")
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "name", Type: "text; drop table users"}); err == nil {
		t.Fatal("unsafe type accepted")
	}
}

func TestRoutineIDRoundTrip(t *testing.T) {
	id := routineID("app", "procedure", "refresh_stats")
	database, routineType, routine, err := parseRoutineID(id)
	if err != nil {
		t.Fatalf("parse routine id: %v", err)
	}
	if database != "app" || routineType != "PROCEDURE" || routine != "refresh_stats" {
		t.Fatalf("unexpected routine identity: %s %s %s", database, routineType, routine)
	}
	if _, _, _, err := parseRoutineID("FUNCTION"); err == nil {
		t.Fatal("accepted non-unique routine id")
	}
}

func TestRedactRowsMasksConfiguredColumns(t *testing.T) {
	rows := []row{{"id": int64(1), "access_token": "plain", "name": "alice"}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["access_token"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
}
