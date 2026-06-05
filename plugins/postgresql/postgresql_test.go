package postgresql

import (
	"context"
	"errors"
	"slices"
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
		t.Fatal("PostgreSQL must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialDBPassword) {
		t.Fatal("database password credential should support PostgreSQL")
	}
	if !plugintest.CredentialKindSupported(m.Config, plugin.CredentialTLSClientCert) {
		t.Fatal("TLS client certificate credential should support PostgreSQL")
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	// Safety gates return before the pool is touched, so a nil pool is fine here.
	_, err := executeQueryRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, nil, sqldb.QueryRequest{Query: "delete from accounts"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, nil, sqldb.QueryRequest{Query: "drop table accounts"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != plugin.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestParseOptionsUsesTLSCredentialAsClientCertificate(t *testing.T) {
	opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
		"host":                          "db.local",
		"database":                      "postgres",
		"auth":                          authClientCert,
		"_auth_client_cert_id_identity": "cert-user",
		"_auth_client_cert_id_secret":   "pem-material",
	}})
	if err != nil {
		t.Fatalf("parse options: %v", err)
	}
	if opts.Username != "cert-user" || opts.Password != "" || opts.ClientCertificate != "pem-material" || opts.TLSMode != "require" {
		t.Fatalf("unexpected credential material: %+v", opts)
	}
}

func TestRedactRowsMasksConfiguredColumnsButKeepsRowKey(t *testing.T) {
	rows := []row{{"id": int64(1), "password": "plain", "name": "alice", "_key": map[string]any{"id": int64(1)}}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["password"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
	if _, ok := rows[0]["_key"].(map[string]any); !ok {
		t.Fatalf("_key must survive redaction so edits stay possible: %#v", rows[0])
	}
}

func TestAttachRowKeysOnlyWithPrimaryKey(t *testing.T) {
	withPK := []row{{"id": int64(7), "name": "a"}}
	attachRowKeys(withPK, []string{"id"}, nil)
	key, ok := withPK[0]["_key"].(map[string]any)
	if !ok || key["id"] != int64(7) {
		t.Fatalf("expected _key from primary key, got %#v", withPK[0])
	}
	keyless := []row{{"name": "a"}}
	attachRowKeys(keyless, nil, nil)
	if _, ok := keyless[0]["_key"]; ok {
		t.Fatal("keyless tables must stay read-only (no _key)")
	}
	// A primary key that is itself sensitive must not be shipped raw via _key.
	secretPK := []row{{"api_key": "live_xyz", "name": "a"}}
	attachRowKeys(secretPK, []string{"api_key"}, sqldb.DefaultRedactColumnPatterns())
	if _, ok := secretPK[0]["_key"]; ok {
		t.Fatal("tables keyed by a sensitive column must stay read-only (no _key leak)")
	}
}

// TestManifestReferencesResolve guards the declarative wiring: every route ID a
// tab, data grid, or action points at must actually exist, or the UI breaks.
func TestManifestReferencesResolve(t *testing.T) {
	p := New()
	m := p.Manifest()
	routeIDs := map[string]bool{}
	for _, r := range p.Routes() {
		routeIDs[r.ID] = true
	}
	actionByID := map[string]plugin.Action{}
	for _, a := range m.Actions {
		actionByID[a.ID] = a
		if !routeIDs[a.RouteID] {
			t.Fatalf("action %q points at missing route %q", a.ID, a.RouteID)
		}
	}
	checkAction := func(id string) {
		if _, ok := actionByID[id]; !ok {
			t.Fatalf("referenced action %q is not declared", id)
		}
	}
	checkSource := func(key string, ds *plugin.DataSource) {
		if ds != nil && !routeIDs[ds.RouteID] {
			t.Fatalf("config %q points at missing route %q", key, ds.RouteID)
		}
	}
	for _, g := range m.Tree {
		if !routeIDs[g.Source.RouteID] {
			t.Fatalf("tree group %q points at missing route %q", g.Key, g.Source.RouteID)
		}
	}
	for _, res := range m.Resources {
		if !routeIDs[res.List.RouteID] {
			t.Fatalf("resource %q list points at missing route %q", res.Kind, res.List.RouteID)
		}
		for _, id := range append(append([]string{}, res.Actions.Toolbar...), res.Actions.Row...) {
			checkAction(id)
		}
		for _, id := range res.Actions.Detail {
			checkAction(id)
		}
		for _, tab := range res.Detail.Tabs {
			if tab.Source != nil && !routeIDs[tab.Source.RouteID] {
				t.Fatalf("resource %q tab %q points at missing route %q", res.Kind, tab.Key, tab.Source.RouteID)
			}
			if tc, ok := tab.Config.(plugin.TableConfig); ok {
				checkSource("insert", tc.Insert)
				checkSource("update", tc.Update)
				checkSource("delete", tc.Delete)
			}
		}
	}
}

// TestTableDataGridIsEditable locks in the headline professional feature: the
// table Data tab is an editable grid wired to row insert/update/delete.
func TestTableDataGridIsEditable(t *testing.T) {
	m := New().Manifest()
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
	if data.Key == "" {
		t.Fatal("table resource is missing a Data tab")
	}
	tc, ok := data.Config.(plugin.TableConfig)
	if !ok || !tc.Editable {
		t.Fatalf("Data tab must be editable: %#v", data.Config)
	}
	for key, ds := range map[string]*plugin.DataSource{"insert": tc.Insert, "update": tc.Update, "delete": tc.Delete} {
		if ds == nil {
			t.Fatalf("Data tab missing %q mutation source: %#v", key, data.Config)
		}
	}
}

func TestDatabaseTablesTabHasCreate(t *testing.T) {
	m := New().Manifest()
	var tab plugin.Panel
	for _, res := range m.Resources {
		if res.Kind != "database" {
			continue
		}
		for _, t := range res.Detail.Tabs {
			if t.Key == "tables" {
				tab = t
			}
		}
	}
	tc, ok := tab.Config.(plugin.TableConfig)
	if !ok || !slices.Contains(tc.ActionIDs, "postgresql.table.create.in_database") {
		t.Fatalf("database Tables tab must offer a create action: %#v", tab.Config)
	}

	var input *plugin.Schema
	for _, r := range New().Routes() {
		if r.ID == "postgresql.table.create.in_database" {
			input = r.Input
		}
	}
	if input == nil {
		t.Fatal("missing postgresql.table.create.in_database route")
	}
	var schema plugin.Field
	for _, f := range input.Groups[0].Fields {
		if f.Key == "schema" {
			schema = f
		}
	}
	if schema.Type != plugin.FieldSelect || schema.OptionsSource == nil {
		t.Fatalf("database-level create form needs a schema picker: %#v", schema)
	}
}

func TestRenameTableSQL(t *testing.T) {
	got, err := renameTableSQL("public", "people", "persons")
	if err != nil {
		t.Fatalf("renameTableSQL: %v", err)
	}
	want := `ALTER TABLE "public"."people" RENAME TO "persons"`
	if got != want {
		t.Fatalf("rename table SQL\n got: %s\nwant: %s", got, want)
	}
	if _, err := renameTableSQL("public", "people", "bad name"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected invalid identifier rejection, got %v", err)
	}
	if _, err := renameTableSQL("public", "people", `x"; DROP TABLE y;--`); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected injection rejection, got %v", err)
	}
}

func TestRenameColumnSQL(t *testing.T) {
	got, err := renameColumnSQL("public", "people", "name", "full_name")
	if err != nil {
		t.Fatalf("renameColumnSQL: %v", err)
	}
	want := `ALTER TABLE "public"."people" RENAME COLUMN "name" TO "full_name"`
	if got != want {
		t.Fatalf("rename column SQL\n got: %s\nwant: %s", got, want)
	}
	if _, err := renameColumnSQL("public", "people", "name", "bad name"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected invalid new name rejection, got %v", err)
	}
	if _, err := renameColumnSQL("public", "people", `n"ame`, "x"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected invalid source column rejection, got %v", err)
	}
}

func TestAlterColumnTypeSQL(t *testing.T) {
	got, err := alterColumnTypeSQL("public", "people", "age", "integer", "")
	if err != nil {
		t.Fatalf("alterColumnTypeSQL: %v", err)
	}
	want := `ALTER TABLE "public"."people" ALTER COLUMN "age" TYPE integer`
	if got != want {
		t.Fatalf("alter column SQL\n got: %s\nwant: %s", got, want)
	}
	got, err = alterColumnTypeSQL("public", "people", "age", "integer", "age::integer")
	if err != nil {
		t.Fatalf("alterColumnTypeSQL with using: %v", err)
	}
	want = `ALTER TABLE "public"."people" ALTER COLUMN "age" TYPE integer USING age::integer`
	if got != want {
		t.Fatalf("alter column USING SQL\n got: %s\nwant: %s", got, want)
	}
	if _, err := alterColumnTypeSQL("public", "people", "age", "integer; DROP TABLE x", ""); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected unsafe type rejection, got %v", err)
	}
	if _, err := alterColumnTypeSQL("public", "people", "age", "integer", "age::int; DROP TABLE x"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected unsafe USING rejection, got %v", err)
	}
	if _, err := alterColumnTypeSQL("public", "people", `a"ge`, "integer", ""); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected invalid column rejection, got %v", err)
	}
}

func TestDropConstraintSQL(t *testing.T) {
	got, err := dropConstraintSQL("public", "people", "people_pkey")
	if err != nil {
		t.Fatalf("dropConstraintSQL: %v", err)
	}
	want := `ALTER TABLE "public"."people" DROP CONSTRAINT "people_pkey"`
	if got != want {
		t.Fatalf("drop constraint SQL\n got: %s\nwant: %s", got, want)
	}
	if _, err := dropConstraintSQL("public", "people", "bad name"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("expected invalid constraint name rejection, got %v", err)
	}
}

func TestAddConstraintSQL(t *testing.T) {
	cases := []struct {
		name string
		req  constraintRequest
		want string
	}{
		{
			name: "primary key",
			req:  constraintRequest{Name: "people_pkey", Type: constraintPrimaryKey, Columns: []any{"id"}},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "people_pkey" PRIMARY KEY ("id")`,
		},
		{
			name: "unique multi-column",
			req:  constraintRequest{Name: "uq_name_email", Type: constraintUnique, Columns: []any{"name", "email"}},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "uq_name_email" UNIQUE ("name", "email")`,
		},
		{
			name: "check",
			req:  constraintRequest{Name: "ck_age", Type: constraintCheck, Check: "age > 0"},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "ck_age" CHECK (age > 0)`,
		},
		{
			name: "foreign key bare table",
			req:  constraintRequest{Name: "fk_org", Type: constraintForeignKey, Columns: []any{"org_id"}, RefTable: "orgs", RefColumns: "id"},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "fk_org" FOREIGN KEY ("org_id") REFERENCES "orgs" ("id")`,
		},
		{
			name: "foreign key qualified with on delete",
			req:  constraintRequest{Name: "fk_org", Type: constraintForeignKey, Columns: []any{"org_id"}, RefTable: "public.orgs", RefColumns: "id", OnDelete: "cascade"},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "fk_org" FOREIGN KEY ("org_id") REFERENCES "public"."orgs" ("id") ON DELETE CASCADE`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := addConstraintSQL("public", "people", tc.req)
			if err != nil {
				t.Fatalf("addConstraintSQL: %v", err)
			}
			if got != tc.want {
				t.Fatalf("add constraint SQL\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}

	bad := []struct {
		name string
		req  constraintRequest
	}{
		{"bad name", constraintRequest{Name: "bad name", Type: constraintPrimaryKey, Columns: []any{"id"}}},
		{"bad column", constraintRequest{Name: "c", Type: constraintPrimaryKey, Columns: []any{`i"d`}}},
		{"unsafe check", constraintRequest{Name: "c", Type: constraintCheck, Check: "1=1; DROP TABLE x"}},
		{"empty check", constraintRequest{Name: "c", Type: constraintCheck}},
		{"fk bad ref table", constraintRequest{Name: "c", Type: constraintForeignKey, Columns: []any{"org_id"}, RefTable: "or gs", RefColumns: "id"}},
		{"fk bad ref column", constraintRequest{Name: "c", Type: constraintForeignKey, Columns: []any{"org_id"}, RefTable: "orgs", RefColumns: `i"d`}},
		{"fk bad on delete", constraintRequest{Name: "c", Type: constraintForeignKey, Columns: []any{"org_id"}, RefTable: "orgs", RefColumns: "id", OnDelete: "explode"}},
		{"unknown type", constraintRequest{Name: "c", Type: "wat", Columns: []any{"id"}}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := addConstraintSQL("public", "people", tc.req); !errors.Is(err, plugin.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestTreeIsDatabaseRooted(t *testing.T) {
	m := New().Manifest()
	if len(m.Tree) != 1 || m.Tree[0].Key != "databases" {
		t.Fatalf("tree should root at a single Databases group, got %#v", m.Tree)
	}
}

func TestTableCreateColumnsIsStructuredArray(t *testing.T) {
	assertColumnsArray(t, New(), "postgresql.table.create", []string{"name", "type", "nullable", "primary", "unique", "default"})
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
