package sqldb

import (
	"reflect"
	"testing"
)

func pgDialect() Dialect {
	return Dialect{QuoteIdent: QuoteIdent, Placeholder: DollarPlaceholder}
}

func TestDialectInsertIsDeterministicAndParameterized(t *testing.T) {
	stmt, args, err := pgDialect().Insert(`"public"."users"`, map[string]any{"email": "a@b.c", "age": float64(30)})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	want := `INSERT INTO "public"."users" ("age", "email") VALUES ($1, $2)`
	if stmt != want {
		t.Fatalf("unexpected insert:\n got %q\nwant %q", stmt, want)
	}
	if !reflect.DeepEqual(args, []any{int64(30), "a@b.c"}) {
		t.Fatalf("unexpected args (integral float must normalize to int64): %#v", args)
	}
}

func TestDialectUpdateOrdersValuesThenKey(t *testing.T) {
	stmt, args, err := pgDialect().Update(`"public"."users"`, map[string]any{"id": float64(7)}, map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	want := `UPDATE "public"."users" SET "name" = $1 WHERE "id" = $2`
	if stmt != want {
		t.Fatalf("unexpected update:\n got %q\nwant %q", stmt, want)
	}
	if !reflect.DeepEqual(args, []any{"x", int64(7)}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestDialectDeleteRequiresKey(t *testing.T) {
	if _, _, err := pgDialect().Delete(`"public"."users"`, nil); err == nil {
		t.Fatal("delete without a key must be rejected so it can never wipe a table")
	}
	stmt, args, err := pgDialect().Delete(`"public"."users"`, map[string]any{"id": float64(7)})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if stmt != `DELETE FROM "public"."users" WHERE "id" = $1` || !reflect.DeepEqual(args, []any{int64(7)}) {
		t.Fatalf("unexpected delete: %q args=%#v", stmt, args)
	}
}

func TestDialectRejectsUnsafeColumnNames(t *testing.T) {
	if _, _, err := pgDialect().Insert(`"t"`, map[string]any{"id; drop table users": 1}); err == nil {
		t.Fatal("unsafe column identifier accepted")
	}
}

func TestDialectMatchesNullKeyWithoutBinding(t *testing.T) {
	stmt, args, err := pgDialect().Delete(`"t"`, map[string]any{"a": nil, "b": float64(2)})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	want := `DELETE FROM "t" WHERE "a" IS NULL AND "b" = $1`
	if stmt != want {
		t.Fatalf("unexpected null-key delete:\n got %q\nwant %q", stmt, want)
	}
	if !reflect.DeepEqual(args, []any{int64(2)}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestStatementSafetyClassification(t *testing.T) {
	readOnly := []string{
		"select * from accounts",
		"WITH q AS (SELECT 1) SELECT * FROM q",
		"show search_path",
		"explain select 1",
	}
	for _, sql := range readOnly {
		if !IsReadOnlyStatement(sql) {
			t.Fatalf("expected read-only statement: %q", sql)
		}
		if IsDestructiveStatement(sql) {
			t.Fatalf("read-only statement classified destructive: %q", sql)
		}
	}
	writes := []string{
		"update accounts set locked = true",
		"delete from accounts",
		"truncate table audit_log",
		"drop table audit_log",
		"alter table users add column enabled boolean",
		"create table demo(id bigint)",
	}
	for _, sql := range writes {
		if IsReadOnlyStatement(sql) {
			t.Fatalf("write statement classified read-only: %q", sql)
		}
		if !IsDestructiveStatement(sql) {
			t.Fatalf("write statement should require confirmation: %q", sql)
		}
	}
}

func TestSplitStatementsKeepsQuotedSemicolons(t *testing.T) {
	got := SplitStatements("select ';' as semi; select 2;")
	if len(got) != 2 {
		t.Fatalf("expected two statements, got %d: %#v", len(got), got)
	}
	if got[0] != "select ';' as semi" || got[1] != "select 2" {
		t.Fatalf("unexpected split: %#v", got)
	}
}

func TestDDLColumnValidation(t *testing.T) {
	if _, err := DDLColumn(ColumnSpec{Name: "name", Type: "text", Nullable: true}); err != nil {
		t.Fatalf("valid column rejected: %v", err)
	}
	if _, err := DDLColumn(ColumnSpec{Name: "bad-name", Type: "text", Nullable: true}); err == nil {
		t.Fatal("invalid identifier accepted")
	}
	if _, err := DDLColumn(ColumnSpec{Name: "name", Type: "text; drop table users", Nullable: true}); err == nil {
		t.Fatal("unsafe type accepted")
	}
	if _, err := DDLColumn(ColumnSpec{Name: "created_at", Type: "timestamptz", Default: "now(); drop table users"}); err == nil {
		t.Fatal("unsafe default accepted")
	}
}

func TestParseDDLColumnsAcceptsJSONTextAreaValue(t *testing.T) {
	cols, err := ParseDDLColumns(`[{"name":"id","type":"bigserial","primary":true},{"name":"email","type":"text","nullable":false}]`)
	if err != nil {
		t.Fatalf("parse columns: %v", err)
	}
	if len(cols) != 2 || cols[0] != `"id" bigserial NOT NULL PRIMARY KEY` {
		t.Fatalf("unexpected columns: %#v", cols)
	}
}

func TestRedactRowsByColumnPattern(t *testing.T) {
	rows := [][]any{{"alice", "plain", "t1"}}
	got := RedactRows([]string{"username", "password_hash", "api_key"}, rows, DefaultRedactColumnPatterns())
	if got[0][0] != "alice" || got[0][1] != RedactedValue || got[0][2] != RedactedValue {
		t.Fatalf("unexpected redaction: %#v", got)
	}
	if rows[0][1] != "plain" {
		t.Fatalf("redaction mutated source rows: %#v", rows)
	}
}

func TestAuditParamsExcludeRawQuery(t *testing.T) {
	params := AuditParams(QueryAudit{
		Query:        "select 'secret literal'",
		Statements:   []string{"select 'secret literal'"},
		ReadOnlyMode: true,
		RowCount:     3,
		ElapsedMS:    9,
	})
	if params["query_sha256"] == "" || params["statement_count"] != "1" || params["first_statement"] != "SELECT" {
		t.Fatalf("unexpected audit params: %#v", params)
	}
	for _, value := range params {
		if value == "select 'secret literal'" {
			t.Fatalf("raw query leaked into audit params: %#v", params)
		}
	}
}
