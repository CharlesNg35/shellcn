package clickhouse

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/plugins/shared/sqldb"
)

func TestClickHousePluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_CLICKHOUSE_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_CLICKHOUSE_INTEGRATION=1 to run against ClickHouse")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = false
	cfg["require_destructive_confirmation"] = true
	cfg["row_limit"] = 50
	cfg["query_timeout"] = "10s"

	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	createdDatabase := "shellcn_it_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := createDatabase(rowMutationRC(ctx, s, nil, map[string]any{"name": createdDatabase, "if_not_exists": true})); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = s.db.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS "+quoteIdent(createdDatabase))
	})
	databases, err := listDatabases(plugin.NewRequestContext(ctx, models.User{}, s, nil, nil, nil))
	if err != nil {
		t.Fatalf("list databases after create: %v", err)
	}
	if !pageHasName(databases.(plugin.Page[row]), createdDatabase) {
		t.Fatalf("created database was not listed: %#v", databases)
	}

	// View drop round-trip: create a view, drop it through the handler, verify gone.
	if _, err := s.db.ExecContext(ctx, "CREATE VIEW "+qualified(createdDatabase, "shellcn_v")+" AS SELECT 1 AS x"); err != nil {
		t.Fatalf("seed view: %v", err)
	}
	if _, err := dropView(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": createdDatabase, "view": "shellcn_v"}, nil, nil)); err != nil {
		t.Fatalf("drop view: %v", err)
	}
	var vcount uint64
	if err := s.db.QueryRowContext(ctx, "SELECT count() FROM system.tables WHERE database = ? AND name = ?", createdDatabase, "shellcn_v").Scan(&vcount); err != nil || vcount != 0 {
		t.Fatalf("expected view dropped, got %d err=%v", vcount, err)
	}

	seedStatements := []string{
		`CREATE TABLE IF NOT EXISTS shellcn_people (
  id UInt64,
  name String,
  access_token String
) ENGINE = MergeTree ORDER BY id`,
		`TRUNCATE TABLE shellcn_people`,
		`INSERT INTO shellcn_people (id, name, access_token) VALUES (1, 'alice', 'secret-token')`,
		`CREATE VIEW IF NOT EXISTS shellcn_people_view AS SELECT name FROM shellcn_people`,
	}
	for _, statement := range seedStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed database: %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = s.db.ExecContext(cleanupCtx, `DROP VIEW IF EXISTS shellcn_people_view`)
		_, _ = s.db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS shellcn_people`)
	})

	rc := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	list, err := listTables(rc)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if !pageHasName(list.(plugin.Page[row]), "shellcn_people") {
		t.Fatalf("created table was not listed: %#v", list)
	}

	rows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": cfg["database"].(string), "table": "shellcn_people"}, nil, nil))
	if err != nil {
		t.Fatalf("table rows: %v", err)
	}
	page := rows.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["access_token"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	// Free-text search filters the data grid server-side (per-column).
	chPeople := map[string]string{"database": cfg["database"].(string), "table": "shellcn_people"}
	chMatch, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, chPeople, url.Values{"filter": {"alice"}}, nil))
	if err != nil {
		t.Fatalf("filtered rows: %v", err)
	}
	if len(chMatch.(plugin.Page[row]).Items) != 1 {
		t.Fatalf("filter 'alice' should match 1 row, got %#v", chMatch.(plugin.Page[row]).Items)
	}
	chMiss, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, chPeople, url.Values{"filter": {"zzz-nomatch"}}, nil))
	if err != nil {
		t.Fatalf("filtered rows (miss): %v", err)
	}
	if len(chMiss.(plugin.Page[row]).Items) != 0 {
		t.Fatalf("filter 'zzz-nomatch' should match 0 rows, got %#v", chMiss.(plugin.Page[row]).Items)
	}

	result, err := executeQueryRequest(ctx, s, sqldb.QueryRequest{Query: `SELECT name, access_token FROM shellcn_people`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][1] != sqldb.RedactedValue {
		t.Fatalf("expected redacted query result, got %#v", result.Rows)
	}

	tableRC := plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": cfg["database"].(string), "table": "shellcn_people"}, nil, nil)
	for name, fn := range map[string]func(*plugin.RequestContext) (any, error){
		"columns":     tableColumnsRoute,
		"indexes":     tableIndexes,
		"constraints": tableConstraints,
		"definition":  tableDefinition,
	} {
		if _, err := fn(tableRC); err != nil {
			t.Fatalf("table %s route: %v", name, err)
		}
	}
	viewRC := plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": cfg["database"].(string), "table": "shellcn_people_view"}, nil, nil)
	if _, err := tableDefinition(viewRC); err != nil {
		t.Fatalf("view definition route: %v", err)
	}

	for name, fn := range map[string]func(*plugin.RequestContext) (any, error){
		"databases":    listDatabases,
		"views":        listViews,
		"dictionaries": listDictionaries,
		"mutations":    listMutations,
		"merges":       listMerges,
		"processes":    listProcesses,
		"users":        listUsers,
		"completion":   completionRoute,
	} {
		if _, err := fn(rc); err != nil {
			t.Fatalf("%s route: %v", name, err)
		}
	}

	// Column/index management via declarative DDL actions.
	db := cfg["database"].(string)
	ddlRC := func(body map[string]any) *plugin.RequestContext {
		raw, _ := json.Marshal(body)
		return plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": db, "table": "shellcn_people"}, nil, raw)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE shellcn_people ADD INDEX ix_name name TYPE set(0) GRANULARITY 1"); err != nil {
		t.Fatalf("seed index: %v", err)
	}
	if _, err := dropIndex(ddlRC(map[string]any{"name": "ix_name"})); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := dropColumn(ddlRC(map[string]any{"column": "access_token"})); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	var cols int
	if err := s.db.QueryRowContext(ctx, "SELECT count() FROM system.columns WHERE database = ? AND table = 'shellcn_people' AND name = 'access_token'", db).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("expected access_token column dropped, got %d err=%v", cols, err)
	}
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_CLICKHOUSE_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_CLICKHOUSE_DSN is not set")
	}
	name := "shellcn-clickhouse-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"--ulimit", "nofile=262144:262144",
		"-e", "CLICKHOUSE_DB=shellcn",
		"-e", "CLICKHOUSE_USER=shellcn",
		"-e", "CLICKHOUSE_PASSWORD=shellcn",
		"-e", "CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1",
		"-p", "127.0.0.1::9000",
		"clickhouse/clickhouse-server:latest")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "9000/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{
		"host":      host,
		"port":      port,
		"database":  "shellcn",
		"username":  "shellcn",
		"password":  "shellcn",
		"tls_mode":  "disable",
		"read_only": false,
	}
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for {
		sess, err := connect(ctx, plugin.ConnectConfig{
			Config: cfg,
			Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
		})
		if err == nil {
			_ = sess.Close()
			return cfg
		}
		lastErr = err
		if time.Now().After(deadline) {
			logs := exec.CommandContext(ctx, "docker", "logs", "--tail", "120", name)
			out, _ := logs.CombinedOutput()
			t.Fatalf("clickhouse container did not become ready: %v\n%s", lastErr, out)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_CLICKHOUSE_DSN: %v", err)
	}
	port := defaultPort
	if u.Port() != "" {
		if port, err = strconv.Atoi(u.Port()); err != nil {
			t.Fatalf("parse DSN port: %v", err)
		}
	}
	password, _ := u.User.Password()
	return map[string]any{
		"host":      u.Hostname(),
		"port":      port,
		"database":  stringDefault(strings.TrimPrefix(u.Path, "/"), "default"),
		"username":  stringDefault(u.User.Username(), "default"),
		"password":  password,
		"tls_mode":  stringDefault(u.Query().Get("tls"), "disable"),
		"read_only": false,
	}
}

func rowMutationRC(ctx context.Context, s *Session, params map[string]string, body map[string]any) *plugin.RequestContext {
	raw, _ := json.Marshal(body)
	return plugin.NewRequestContext(ctx, models.User{ID: "u1"}, s, params, nil, raw)
}

func run(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func pageHasName(page plugin.Page[row], name string) bool {
	for _, item := range page.Items {
		if item["name"] == name {
			return true
		}
	}
	return false
}
