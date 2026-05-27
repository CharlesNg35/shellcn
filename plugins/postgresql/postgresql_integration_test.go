package postgresql

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestPostgreSQLPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_POSTGRESQL_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_POSTGRESQL_INTEGRATION=1 to run against PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
	pool, err := s.poolFor(ctx, "")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS public.shellcn_people (
  id bigserial PRIMARY KEY,
  name text NOT NULL,
  password text NOT NULL
);
TRUNCATE public.shellcn_people;
INSERT INTO public.shellcn_people (name, password) VALUES ('alice', 'secret-password')`); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DROP TABLE IF EXISTS public.shellcn_people`)
	})

	rc := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	list, err := listTables(rc)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if !pageHasName(list.(plugin.Page[row]), "shellcn_people") {
		t.Fatalf("created table was not listed: %#v", list)
	}

	rows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"schema": "public", "table": "shellcn_people"}, nil, nil))
	if err != nil {
		t.Fatalf("table rows: %v", err)
	}
	page := rows.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["password"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	result, err := executeQueryRequest(ctx, s, pool, sqldb.QueryRequest{Query: `SELECT name, password FROM public.shellcn_people`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][1] != sqldb.RedactedValue {
		t.Fatalf("expected redacted query result, got %#v", result.Rows)
	}

	// Editable data grid: rows carry _key from the primary key.
	if key, ok := page.Items[0]["_key"].(map[string]any); !ok || key["id"] == nil {
		t.Fatalf("table rows must carry a _key from the primary key: %#v", page.Items[0])
	}

	// Insert → update → delete a row through the grid's mutation routes.
	tableParams := map[string]string{"schema": "public", "table": "shellcn_people"}
	if _, err := insertRow(rowMutationRC(ctx, s, tableParams, map[string]any{"values": map[string]any{"name": "bob", "password": "pw"}})); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	var bobID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM public.shellcn_people WHERE name = 'bob'`).Scan(&bobID); err != nil {
		t.Fatalf("read inserted row: %v", err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, tableParams, map[string]any{"key": map[string]any{"id": bobID}, "values": map[string]any{"name": "bob2"}})); err != nil {
		t.Fatalf("update row: %v", err)
	}
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM public.shellcn_people WHERE id = $1`, bobID).Scan(&name); err != nil || name != "bob2" {
		t.Fatalf("expected updated name bob2, got %q err=%v", name, err)
	}
	if _, err := deleteRow(rowMutationRC(ctx, s, tableParams, map[string]any{"key": map[string]any{"id": bobID}})); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.shellcn_people`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("expected 1 row after delete, got %d err=%v", remaining, err)
	}

	// A non-primary-key key must be refused server-side (no mass update).
	if _, err := updateRow(rowMutationRC(ctx, s, tableParams, map[string]any{"key": map[string]any{"name": "alice"}, "values": map[string]any{"password": "x"}})); err == nil {
		t.Fatal("update with a non-primary-key key must be rejected")
	}

	// Cover the table structure + catalog routes the UI drives.
	structRoutes := map[string]func(*plugin.RequestContext) (any, error){
		"columns": tableColumnsRoute, "indexes": tableIndexes,
		"constraints": tableConstraints, "ddl": tableDDL,
	}
	for label, fn := range structRoutes {
		if _, err := fn(plugin.NewRequestContext(ctx, models.User{}, s, tableParams, nil, nil)); err != nil {
			t.Fatalf("%s route: %v", label, err)
		}
	}
	catalogRoutes := map[string]func(*plugin.RequestContext) (any, error){
		"databases": listDatabases, "schemas": listSchemas, "views": listViews,
		"functions": listFunctions, "sequences": listSequences, "completion": completionRoute,
	}
	for label, fn := range catalogRoutes {
		if _, err := fn(rc); err != nil {
			t.Fatalf("%s route: %v", label, err)
		}
	}

	// Column/index management via declarative DDL actions.
	if _, err := createIndex(rowMutationRC(ctx, s, tableParams, map[string]any{"name": "ix_people_name", "columns": "name", "unique": false})); err != nil {
		t.Fatalf("create index: %v", err)
	}
	var idx int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname='public' AND tablename='shellcn_people' AND indexname='ix_people_name'`).Scan(&idx); err != nil || idx != 1 {
		t.Fatalf("expected created index, got %d err=%v", idx, err)
	}
	if _, err := dropIndex(rowMutationRC(ctx, s, tableParams, map[string]any{"name": "ix_people_name"})); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := dropColumn(rowMutationRC(ctx, s, tableParams, map[string]any{"column": "password"})); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	var cols int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='shellcn_people' AND column_name='password'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("expected password column dropped, got %d err=%v", cols, err)
	}

	// DROP DATABASE must succeed even after the connection has browsed (and
	// cached a pool to) the target database.
	if _, err := createDatabase(rowMutationRC(ctx, s, nil, map[string]any{"name": "shellcn_droptest"})); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if _, err := listSchemas(plugin.NewRequestContext(ctx, models.User{}, s, nil, url.Values{"p.database": {"shellcn_droptest"}}, nil)); err != nil {
		t.Fatalf("browse new database: %v", err)
	}
	if _, err := dropDatabase(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": "shellcn_droptest"}, nil, nil)); err != nil {
		t.Fatalf("drop database after browsing: %v", err)
	}
}

func rowMutationRC(ctx context.Context, s *Session, params map[string]string, body map[string]any) *plugin.RequestContext {
	raw, _ := json.Marshal(body)
	return plugin.NewRequestContext(ctx, models.User{ID: "u1"}, s, params, nil, raw)
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_POSTGRESQL_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_POSTGRESQL_DSN is not set")
	}
	name := "shellcn-postgres-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "POSTGRES_PASSWORD=shellcn",
		"-e", "POSTGRES_DB=shellcn",
		"-p", "127.0.0.1::5432",
		"postgres:16-alpine")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "5432/tcp")
	host, portText, ok := strings.Cut(strings.TrimSpace(out), ":")
	if !ok {
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
		"username":  "postgres",
		"password":  "shellcn",
		"tls_mode":  "disable",
		"read_only": false,
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		sess, err := connect(ctx, plugin.ConnectConfig{
			Config: cfg,
			Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
		})
		if err == nil {
			_ = sess.Close()
			return cfg
		}
		if time.Now().After(deadline) {
			t.Fatalf("postgres container did not become ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_POSTGRESQL_DSN: %v", err)
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
		"database":  strings.TrimPrefix(u.Path, "/"),
		"username":  u.User.Username(),
		"password":  password,
		"tls_mode":  stringDefault(u.Query().Get("sslmode"), "disable"),
		"read_only": false,
	}
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
