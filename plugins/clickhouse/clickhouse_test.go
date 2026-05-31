package clickhouse

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
		t.Fatalf("register ClickHouse plugin: %v", err)
	}
	m, ok := reg.Manifest(protocolName)
	if !ok {
		t.Fatal("manifest not registered")
	}
	if m.Agent != nil {
		t.Fatal("ClickHouse must not declare agent transport")
	}
	if len(m.SupportedTransports) != 1 || m.SupportedTransports[0] != plugin.TransportDirect {
		t.Fatalf("unexpected transports: %+v", m.SupportedTransports)
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialDBPassword, protocolName) {
		t.Fatal("database password credential should support ClickHouse")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialTLSClientCert, protocolName) {
		t.Fatal("TLS client certificate credential should support ClickHouse")
	}
}

func TestQuerySafetyStopsBeforeDatabase(t *testing.T) {
	_, err := executeQueryRequest(context.Background(), &Session{opts: options{ReadOnly: true}}, sqldb.QueryRequest{Query: "insert into events values (1)"})
	if !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("expected read-only forbidden error, got %v", err)
	}
	_, err = executeQueryRequest(context.Background(), &Session{opts: options{RequireConfirm: true}}, sqldb.QueryRequest{Query: "system reload dictionaries"})
	var confirmErr confirmationError
	if !errors.As(err, &confirmErr) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if got := queryAuditResult(err); got != models.AuditDenied {
		t.Fatalf("confirmation should audit as denied, got %s", got)
	}
}

func TestClickHouseDDLColumnValidation(t *testing.T) {
	col, err := ddlColumn(sqldb.ColumnSpec{Name: "event_time", Type: "DateTime"})
	if err != nil {
		t.Fatalf("valid column rejected: %v", err)
	}
	if col != "`event_time` DateTime" {
		t.Fatalf("unexpected column: %q", col)
	}
	col, err = ddlColumn(sqldb.ColumnSpec{Name: "email", Type: "String", Nullable: true, Default: "''"})
	if err != nil {
		t.Fatalf("valid nullable column rejected: %v", err)
	}
	if col != "`email` Nullable(String) DEFAULT ''" {
		t.Fatalf("unexpected nullable column: %q", col)
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "bad-name", Type: "String"}); err == nil {
		t.Fatal("invalid identifier accepted")
	}
	if _, err := ddlColumn(sqldb.ColumnSpec{Name: "name", Type: "String; drop table users"}); err == nil {
		t.Fatal("unsafe type accepted")
	}
}

func TestRedactRowsMasksConfiguredColumns(t *testing.T) {
	rows := []row{{"id": uint64(1), "access_token": "plain", "name": "alice"}}
	redactRows(rows, sqldb.DefaultRedactColumnPatterns())
	if rows[0]["access_token"] != sqldb.RedactedValue || rows[0]["name"] != "alice" {
		t.Fatalf("unexpected row redaction: %#v", rows)
	}
}

func TestInsertRowStatement(t *testing.T) {
	stmt, args, err := dialect.Insert(qualified("analytics", "events"), map[string]any{"id": int64(7), "name": "click"})
	if err != nil {
		t.Fatalf("insert build failed: %v", err)
	}
	want := "INSERT INTO `analytics`.`events` (`id`, `name`) VALUES (?, ?)"
	if stmt != want {
		t.Fatalf("unexpected insert statement:\n got %q\nwant %q", stmt, want)
	}
	if len(args) != 2 || args[0] != int64(7) || args[1] != "click" {
		t.Fatalf("unexpected insert args: %#v", args)
	}
}

func TestAlterUpdateMutationStatement(t *testing.T) {
	stmt, args, err := buildAlterUpdate(qualified("analytics", "events"),
		map[string]any{"id": int64(7)}, map[string]any{"name": "view", "count": int64(3)})
	if err != nil {
		t.Fatalf("update build failed: %v", err)
	}
	want := "ALTER TABLE `analytics`.`events` UPDATE `count` = ?, `name` = ? WHERE `id` = ?"
	if stmt != want {
		t.Fatalf("unexpected update statement:\n got %q\nwant %q", stmt, want)
	}
	if len(args) != 3 || args[0] != int64(3) || args[1] != "view" || args[2] != int64(7) {
		t.Fatalf("unexpected update args: %#v", args)
	}
}

func TestAlterDeleteMutationStatement(t *testing.T) {
	stmt, args, err := buildAlterDelete(qualified("analytics", "events"), map[string]any{"id": int64(7), "shard": "a"})
	if err != nil {
		t.Fatalf("delete build failed: %v", err)
	}
	want := "ALTER TABLE `analytics`.`events` DELETE WHERE `id` = ? AND `shard` = ?"
	if stmt != want {
		t.Fatalf("unexpected delete statement:\n got %q\nwant %q", stmt, want)
	}
	if len(args) != 2 || args[0] != int64(7) || args[1] != "a" {
		t.Fatalf("unexpected delete args: %#v", args)
	}
}

func TestMutationNullKeyMatchesIsNull(t *testing.T) {
	stmt, args, err := buildAlterDelete(qualified("db", "t"), map[string]any{"k": nil})
	if err != nil {
		t.Fatalf("delete build failed: %v", err)
	}
	if stmt != "ALTER TABLE `db`.`t` DELETE WHERE `k` IS NULL" {
		t.Fatalf("unexpected null-key statement: %q", stmt)
	}
	if len(args) != 0 {
		t.Fatalf("null-key match must bind no args, got %#v", args)
	}
}

func TestMutationRejectsEmptyKeyAndValues(t *testing.T) {
	if _, _, err := buildAlterDelete(qualified("db", "t"), nil); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("delete without key must be rejected, got %v", err)
	}
	if _, _, err := buildAlterUpdate(qualified("db", "t"), nil, map[string]any{"a": 1}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("update without key must be rejected, got %v", err)
	}
	if _, _, err := buildAlterUpdate(qualified("db", "t"), map[string]any{"id": 1}, nil); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("update without values must be rejected, got %v", err)
	}
}

func TestMutationRejectsUnsafeIdentifier(t *testing.T) {
	if _, _, err := buildAlterUpdate(qualified("db", "t"), map[string]any{"id": 1}, map[string]any{"bad-col": 1}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe set column must be rejected, got %v", err)
	}
	if _, _, err := buildAlterDelete(qualified("db", "t"), map[string]any{"bad key": 1}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe key column must be rejected, got %v", err)
	}
}

func TestAddIndexStatement(t *testing.T) {
	clause, err := buildAddIndex("ix_value", "value", "minmax", 4)
	if err != nil {
		t.Fatalf("add index build failed: %v", err)
	}
	if clause != "ADD INDEX `ix_value` value TYPE minmax GRANULARITY 4" {
		t.Fatalf("unexpected add index clause: %q", clause)
	}
	// Non-positive granularity defaults to 1; set(0) is a valid index type.
	clause, err = buildAddIndex("ix_name", "lower(name)", "set(0)", 0)
	if err != nil {
		t.Fatalf("add index build failed: %v", err)
	}
	if clause != "ADD INDEX `ix_name` lower(name) TYPE set(0) GRANULARITY 1" {
		t.Fatalf("unexpected add index clause: %q", clause)
	}
}

func TestAddIndexRejectsUnsafeInput(t *testing.T) {
	if _, err := buildAddIndex("bad-name", "value", "minmax", 1); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe index name must be rejected, got %v", err)
	}
	if _, err := buildAddIndex("ix", "value; DROP TABLE t", "minmax", 1); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe index expression must be rejected, got %v", err)
	}
	if _, err := buildAddIndex("ix", "value", "minmax; DROP TABLE t", 1); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe index type must be rejected, got %v", err)
	}
	if _, err := buildAddIndex("ix", "", "minmax", 1); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("empty index expression must be rejected, got %v", err)
	}
}

func TestAddConstraintStatement(t *testing.T) {
	clause, err := buildAddConstraint("age_positive", "age >= 0")
	if err != nil {
		t.Fatalf("add constraint build failed: %v", err)
	}
	if clause != "ADD CONSTRAINT `age_positive` CHECK age >= 0" {
		t.Fatalf("unexpected add constraint clause: %q", clause)
	}
}

func TestAddConstraintRejectsUnsafeInput(t *testing.T) {
	if _, err := buildAddConstraint("bad name", "age >= 0"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe constraint name must be rejected, got %v", err)
	}
	if _, err := buildAddConstraint("c", "age >= 0; DROP TABLE t"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unsafe constraint expression must be rejected, got %v", err)
	}
	if _, err := buildAddConstraint("c", ""); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("empty constraint expression must be rejected, got %v", err)
	}
}

func TestModifyColumnReusesDDLColumn(t *testing.T) {
	// ALTER ... MODIFY COLUMN reuses ddlColumn, so the rendered column definition
	// matches the ADD COLUMN form, including the Nullable() wrap and DEFAULT.
	col, err := ddlColumn(sqldb.ColumnSpec{Name: "score", Type: "Float64", Default: "0"})
	if err != nil {
		t.Fatalf("valid modify column rejected: %v", err)
	}
	if col != "`score` Float64 DEFAULT 0" {
		t.Fatalf("unexpected modify column: %q", col)
	}
}

func TestReadOnlyBlocksRowMutation(t *testing.T) {
	if err := ensureWritable(&Session{opts: options{ReadOnly: true}}); !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("read-only mode must block row mutations, got %v", err)
	}
}

func TestNoSortingKeyKeepsGridReadOnly(t *testing.T) {
	rows := []row{{"name": "click", "value": int64(1)}}
	attachRowKeys(rows, nil, nil) // no sorting key -> no _key attached
	if _, ok := rows[0]["_key"]; ok {
		t.Fatal("rows without a sorting key must not carry _key")
	}
	// A key column that is itself sensitive also keeps the grid read-only.
	rows = []row{{"token": "abc", "value": int64(1)}}
	attachRowKeys(rows, []string{"token"}, sqldb.DefaultRedactColumnPatterns())
	if _, ok := rows[0]["_key"]; ok {
		t.Fatal("rows with a sensitive key column must not carry _key")
	}
	// A real sorting key produces a _key map mutations can echo back.
	rows = []row{{"id": int64(9), "value": int64(1)}}
	attachRowKeys(rows, []string{"id"}, sqldb.DefaultRedactColumnPatterns())
	key, ok := rows[0]["_key"].(map[string]any)
	if !ok || key["id"] != int64(9) {
		t.Fatalf("expected _key with sorting key value, got %#v", rows[0]["_key"])
	}
	// Such a key must pass row-key validation against the sorting key.
	if err := sqldb.ValidateRowKey([]string{"id"}, key); err != nil {
		t.Fatalf("sorting-key row key should validate: %v", err)
	}
	if err := sqldb.ValidateRowKey(nil, key); !errors.Is(err, plugin.ErrForbidden) {
		t.Fatalf("validation against an empty sorting key must be forbidden, got %v", err)
	}
}
