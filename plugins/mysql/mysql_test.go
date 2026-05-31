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
	if !contains(database.Actions.Toolbar, "mysql.database.create") {
		t.Fatalf("database list actions = %#v, want create database", database.Actions.Toolbar)
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

func TestAlterColumnSQL(t *testing.T) {
	modify, err := alterColumnSQL("`app`.`people`", "name", "", sqldb.ColumnSpec{Type: "varchar(128)", Nullable: false})
	if err != nil {
		t.Fatalf("modify column rejected: %v", err)
	}
	if modify != "ALTER TABLE `app`.`people` MODIFY COLUMN `name` varchar(128) NOT NULL" {
		t.Fatalf("unexpected modify SQL: %q", modify)
	}
	change, err := alterColumnSQL("`app`.`people`", "name", "full_name", sqldb.ColumnSpec{Type: "varchar(255)", Nullable: true, Default: "''"})
	if err != nil {
		t.Fatalf("change column rejected: %v", err)
	}
	if change != "ALTER TABLE `app`.`people` CHANGE COLUMN `name` `full_name` varchar(255) DEFAULT ''" {
		t.Fatalf("unexpected change SQL: %q", change)
	}
	if _, err := alterColumnSQL("`app`.`people`", "name", "bad-name", sqldb.ColumnSpec{Type: "text"}); err == nil {
		t.Fatal("invalid rename identifier accepted")
	}
	if _, err := alterColumnSQL("`app`.`people`", "name", "", sqldb.ColumnSpec{Type: "text; drop table x"}); err == nil {
		t.Fatal("unsafe type accepted")
	}
}

func TestConstraintAddClause(t *testing.T) {
	pk, err := constraintAddClause("app", constraintAddRequest{Kind: "PRIMARY KEY", Columns: []string{"id"}})
	if err != nil || pk != "PRIMARY KEY (`id`)" {
		t.Fatalf("primary key clause = %q err=%v", pk, err)
	}
	uniq, err := constraintAddClause("app", constraintAddRequest{Kind: "UNIQUE", Name: "uq_email", Columns: "email"})
	if err != nil || uniq != "CONSTRAINT `uq_email` UNIQUE (`email`)" {
		t.Fatalf("unique clause = %q err=%v", uniq, err)
	}
	fk, err := constraintAddClause("app", constraintAddRequest{Kind: "FOREIGN KEY", Name: "fk_person", Columns: "person_id", RefTable: "people", RefColumns: "id"})
	if err != nil || fk != "CONSTRAINT `fk_person` FOREIGN KEY (`person_id`) REFERENCES `app`.`people` (`id`)" {
		t.Fatalf("foreign key clause = %q err=%v", fk, err)
	}
	fkRef, err := constraintAddClause("app", constraintAddRequest{Kind: "FOREIGN KEY", Columns: "person_id", RefDatabase: "other", RefTable: "people", RefColumns: "id"})
	if err != nil || fkRef != "FOREIGN KEY (`person_id`) REFERENCES `other`.`people` (`id`)" {
		t.Fatalf("foreign key (cross-db, unnamed) clause = %q err=%v", fkRef, err)
	}
	chk, err := constraintAddClause("app", constraintAddRequest{Kind: "CHECK", Name: "ck_price", Expression: "price > 0"})
	if err != nil || chk != "CONSTRAINT `ck_price` CHECK (price > 0)" {
		t.Fatalf("check clause = %q err=%v", chk, err)
	}
	if _, err := constraintAddClause("app", constraintAddRequest{Kind: "CHECK", Expression: "price > 0; drop table x"}); err == nil {
		t.Fatal("unsafe check expression accepted")
	}
	if _, err := constraintAddClause("app", constraintAddRequest{Kind: "FOREIGN KEY", Columns: "person_id", RefColumns: "id"}); err == nil {
		t.Fatal("foreign key without referenced table accepted")
	}
	if _, err := constraintAddClause("app", constraintAddRequest{Kind: "TRIGGER"}); err == nil {
		t.Fatal("unsupported constraint kind accepted")
	}
}

func TestConstraintDropClause(t *testing.T) {
	cases := map[string]string{
		"PRIMARY KEY": "DROP PRIMARY KEY",
		"FOREIGN KEY": "DROP FOREIGN KEY `fk_person`",
		"CHECK":       "DROP CHECK `fk_person`",
		"UNIQUE":      "DROP INDEX `fk_person`",
	}
	for constraintType, want := range cases {
		got, err := constraintDropClause(constraintType, "fk_person")
		if err != nil || got != want {
			t.Fatalf("drop %q = %q err=%v, want %q", constraintType, got, err, want)
		}
	}
	if _, err := constraintDropClause("WHATEVER", "x"); err == nil {
		t.Fatal("unsupported constraint type accepted for drop")
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
	if tc.ColumnsSource == nil {
		t.Fatal("Data tab missing columnsSource for empty editable tables")
	}
	if tc.ColumnsSource.RouteID != "mysql.table.columns" {
		t.Fatalf("unexpected columnsSource route: %q", tc.ColumnsSource.RouteID)
	}
}

func TestQuoteLiteralEscapes(t *testing.T) {
	cases := map[string]string{
		`alice`:      `'alice'`,
		`o'brien`:    `'o\'brien'`,
		`a\b`:        `'a\\b'`,
		`x'; DROP--`: `'x\'; DROP--'`,
	}
	for in, want := range cases {
		if got := quoteLiteral(in); got != want {
			t.Fatalf("quoteLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUserSpecQuotesAndDefaultsHost(t *testing.T) {
	account, err := userSpec("alice", "")
	if err != nil || account != `'alice'@'%'` {
		t.Fatalf("userSpec default host = %q err=%v", account, err)
	}
	account, err = userSpec("svc", "10.0.0.1")
	if err != nil || account != `'svc'@'10.0.0.1'` {
		t.Fatalf("userSpec = %q err=%v", account, err)
	}
	// A username carrying a quote must be escaped, not allowed to break out.
	account, err = userSpec("ev'il", "%")
	if err != nil || account != `'ev\'il'@'%'` {
		t.Fatalf("userSpec malicious = %q err=%v", account, err)
	}
	if _, err := userSpec("", "%"); err == nil {
		t.Fatal("empty username accepted")
	}
}

func TestGrantClause(t *testing.T) {
	clause, err := grantClause([]string{"SELECT", "INSERT"}, "app.users")
	if err != nil || clause != "SELECT, INSERT ON `app`.`users`" {
		t.Fatalf("grant clause = %q err=%v", clause, err)
	}
	// Default scope and comma-separated string input.
	clause, err = grantClause("select", "")
	if err != nil || clause != "SELECT ON *.*" {
		t.Fatalf("grant clause (default scope) = %q err=%v", clause, err)
	}
	clause, err = grantClause([]any{"SELECT"}, "app.*")
	if err != nil || clause != "SELECT ON `app`.*" {
		t.Fatalf("grant clause (db scope) = %q err=%v", clause, err)
	}
	// Duplicates collapse.
	clause, err = grantClause([]string{"SELECT", "select"}, "*.*")
	if err != nil || clause != "SELECT ON *.*" {
		t.Fatalf("grant clause (dedup) = %q err=%v", clause, err)
	}
	if _, err := grantClause([]string{}, "*.*"); err == nil {
		t.Fatal("empty privilege list accepted")
	}
	// Privileges outside the allowlist (incl. injection attempts) are rejected.
	if _, err := grantClause([]string{"SELECT; DROP DATABASE app"}, "*.*"); err == nil {
		t.Fatal("unsafe privilege accepted")
	}
	if _, err := grantClause([]string{"SUPER"}, "*.*"); err == nil {
		t.Fatal("non-allowlisted privilege accepted")
	}
	// Scope identifiers are validated; injection in the scope is rejected.
	if _, err := grantClause([]string{"SELECT"}, "app`; DROP--.*"); err == nil {
		t.Fatal("unsafe scope identifier accepted")
	}
	if _, err := grantClause([]string{"SELECT"}, "noseparator"); err == nil {
		t.Fatal("scope without database.object separator accepted")
	}
}

func TestUserActionsWiredToRoutes(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	for _, id := range []string{"mysql.user.create", "mysql.user.grant", "mysql.user.drop"} {
		if !routeIDs[id] {
			t.Fatalf("missing route %q", id)
		}
	}
	actions := map[string]plugin.Action{}
	for _, a := range m.Actions {
		actions[a.ID] = a
	}
	if a, ok := actions["mysql.user.drop"]; !ok || !a.Confirm {
		t.Fatalf("drop user action must require confirmation: %#v", a)
	}
	var user plugin.ResourceType
	for _, res := range m.Resources {
		if res.Kind == "user" {
			user = res
		}
	}
	if !contains(user.Actions.Toolbar, "mysql.user.create") {
		t.Fatalf("user toolbar = %#v, want create", user.Actions.Toolbar)
	}
	if !contains(user.Actions.Row, "mysql.user.drop") {
		t.Fatalf("user row actions = %#v, want drop", user.Actions.Row)
	}
	if contains(user.Actions.Row, "mysql.user.grant") {
		t.Fatalf("user row actions = %#v, grant belongs in detail not row", user.Actions.Row)
	}
	if !contains(user.Actions.Detail, "mysql.user.grant") || !contains(user.Actions.Detail, "mysql.user.drop") {
		t.Fatalf("user detail actions = %#v, want grant+drop", user.Actions.Detail)
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

func TestTableCreateColumnsIsStructuredArray(t *testing.T) {
	assertColumnsArray(t, New(), "mysql.table.create", []string{"name", "type", "nullable", "primary", "unique", "default"})
}

func assertColumnsArray(t *testing.T, p plugin.Plugin, routeID string, wantKeys []string) {
	t.Helper()
	var schema *plugin.Schema
	for _, r := range p.Routes() {
		if r.ID == routeID {
			schema = r.Input
			break
		}
	}
	if schema == nil {
		t.Fatalf("route %q has no input schema", routeID)
	}
	var columns *plugin.Field
	for _, g := range schema.Groups {
		for i := range g.Fields {
			if g.Fields[i].Key == "columns" {
				columns = &g.Fields[i]
			}
		}
	}
	if columns == nil {
		t.Fatalf("%s: no columns field", routeID)
	}
	if columns.Type != plugin.FieldArray {
		t.Fatalf("%s: columns is %q, want array", routeID, columns.Type)
	}
	if columns.Item == nil || columns.Item.Type != plugin.FieldObject {
		t.Fatalf("%s: columns item is not an object", routeID)
	}
	got := make([]string, 0, len(columns.Item.Fields))
	for _, f := range columns.Item.Fields {
		got = append(got, f.Key)
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("%s: columns item keys = %v, want %v", routeID, got, wantKeys)
	}
	for i, k := range wantKeys {
		if got[i] != k {
			t.Fatalf("%s: columns item keys = %v, want %v", routeID, got, wantKeys)
		}
	}
}
