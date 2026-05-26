package sqldb

import "testing"

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
