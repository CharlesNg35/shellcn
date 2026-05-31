package cockroachdb

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
		t.Fatalf("register CockroachDB plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("CockroachDB must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support CockroachDB")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support CockroachDB")
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

func TestRedactRowsMasksConfiguredColumns(t *testing.T) {
	rows := []row{{"id": int64(1), "password": "plain", "name": "alice"}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["password"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
}

func TestRenameTableSQL(t *testing.T) {
	got, err := renameTableSQL("public", "people", "humans")
	if err != nil {
		t.Fatalf("renameTableSQL: %v", err)
	}
	if want := `ALTER TABLE "public"."people" RENAME TO "humans"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := renameTableSQL("public", "people", "bad name"); err == nil {
		t.Fatal("renameTableSQL must reject an unsafe new name")
	}
}

func TestRenameColumnSQL(t *testing.T) {
	got, err := renameColumnSQL("public", "people", "name", "full_name")
	if err != nil {
		t.Fatalf("renameColumnSQL: %v", err)
	}
	if want := `ALTER TABLE "public"."people" RENAME COLUMN "name" TO "full_name"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := renameColumnSQL("public", "people", "name", "1bad"); err == nil {
		t.Fatal("renameColumnSQL must reject an unsafe new name")
	}
}

func TestAlterColumnTypeSQL(t *testing.T) {
	got, err := alterColumnTypeSQL("public", "people", "age", "INT8", "")
	if err != nil {
		t.Fatalf("alterColumnTypeSQL: %v", err)
	}
	if want := `ALTER TABLE "public"."people" ALTER COLUMN "age" TYPE INT8`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	withUsing, err := alterColumnTypeSQL("public", "people", "age", "INT8", "age::INT8")
	if err != nil {
		t.Fatalf("alterColumnTypeSQL using: %v", err)
	}
	if want := `ALTER TABLE "public"."people" ALTER COLUMN "age" TYPE INT8 USING age::INT8`; withUsing != want {
		t.Fatalf("got %q, want %q", withUsing, want)
	}
	if _, err := alterColumnTypeSQL("public", "people", "age", "INT8; DROP TABLE people", ""); err == nil {
		t.Fatal("alterColumnTypeSQL must reject an unsafe type")
	}
}

func TestDropConstraintSQL(t *testing.T) {
	got, err := dropConstraintSQL("public", "people", "people_pkey")
	if err != nil {
		t.Fatalf("dropConstraintSQL: %v", err)
	}
	if want := `ALTER TABLE "public"."people" DROP CONSTRAINT "people_pkey"`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := dropConstraintSQL("public", "people", "bad name"); err == nil {
		t.Fatal("dropConstraintSQL must reject an unsafe name")
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
			req:  constraintRequest{Name: "people_pk", Type: constraintPrimaryKey, Columns: []any{"id"}},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "people_pk" PRIMARY KEY ("id")`,
		},
		{
			name: "unique",
			req:  constraintRequest{Name: "people_email_uq", Type: constraintUnique, Columns: []any{"email"}},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "people_email_uq" UNIQUE ("email")`,
		},
		{
			name: "check",
			req:  constraintRequest{Name: "people_age_ck", Type: constraintCheck, Check: "age > 0"},
			want: `ALTER TABLE "public"."people" ADD CONSTRAINT "people_age_ck" CHECK (age > 0)`,
		},
		{
			name: "foreign key",
			req:  constraintRequest{Name: "orders_fk", Type: constraintForeignKey, Columns: []any{"person_id"}, RefTable: "public.people", RefColumns: "id", OnDelete: "cascade"},
			want: `ALTER TABLE "public"."orders" ADD CONSTRAINT "orders_fk" FOREIGN KEY ("person_id") REFERENCES "public"."people" ("id") ON DELETE CASCADE`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := "people"
			if tc.req.Type == constraintForeignKey {
				table = "orders"
			}
			got, err := addConstraintSQL("public", table, tc.req)
			if err != nil {
				t.Fatalf("addConstraintSQL: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	if _, err := addConstraintSQL("public", "people", constraintRequest{Name: "x", Type: "nope"}); err == nil {
		t.Fatal("addConstraintSQL must reject an unknown constraint type")
	}
	if _, err := addConstraintSQL("public", "people", constraintRequest{Name: "x", Type: constraintCheck, Check: "1; DROP TABLE people"}); err == nil {
		t.Fatal("addConstraintSQL must reject an unsafe check expression")
	}
	if _, err := addConstraintSQL("public", "orders", constraintRequest{Name: "x", Type: constraintForeignKey, Columns: []any{"person_id"}, RefTable: "public.people", RefColumns: "id", OnDelete: "BOGUS"}); err == nil {
		t.Fatal("addConstraintSQL must reject an unsupported ON DELETE action")
	}
}
