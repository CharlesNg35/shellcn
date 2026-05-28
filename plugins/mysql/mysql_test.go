package mysql

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

	routeIDs := map[string]bool{}
	for _, route := range New().Routes() {
		routeIDs[route.ID] = true
	}
	if !routeIDs["mysql.database.create"] {
		t.Fatal("MySQL should expose a create database route")
	}
	actions := map[string]plugin.Action{}
	for _, action := range m.Actions {
		actions[action.ID] = action
	}
	if action := actions["mysql.database.create"]; action.RouteID != "mysql.database.create" {
		t.Fatalf("create database action is not wired to its route: %#v", action)
	}
	var database plugin.ResourceType
	for _, res := range m.Resources {
		if res.Kind == "database" {
			database = res
			break
		}
	}
	if !contains(database.ListActionIDs, "mysql.database.create") {
		t.Fatalf("database list actions = %#v, want create database", database.ListActionIDs)
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
	rows := []row{{"id": int64(1), "access_token": "plain", "name": "alice", "_key": map[string]any{"id": int64(1)}}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["access_token"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
	if _, ok := rows[0]["_key"].(map[string]any); !ok {
		t.Fatalf("_key must survive redaction: %#v", rows[0])
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
	if data.Key == "" || data.Config["editable"] != true {
		t.Fatalf("table Data tab must be an editable grid: %#v", data.Config)
	}
	for _, key := range []string{"insert", "update", "delete"} {
		ds, ok := data.Config[key].(*plugin.DataSource)
		if !ok {
			t.Fatalf("Data tab missing %q mutation source", key)
		}
		if !routeIDs[ds.RouteID] {
			t.Fatalf("Data tab %q points at missing route %q", key, ds.RouteID)
		}
	}
	columnsSource, ok := data.Config["columnsSource"].(*plugin.DataSource)
	if !ok {
		t.Fatal("Data tab missing columnsSource for empty editable tables")
	}
	if columnsSource.RouteID != "mysql.table.columns" {
		t.Fatalf("unexpected columnsSource route: %q", columnsSource.RouteID)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
