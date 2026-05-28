package cockroachdb

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

func TestCockroachDBPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_COCKROACHDB_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_COCKROACHDB_INTEGRATION=1 to run against CockroachDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
		_, _ = s.pool.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+sqldb.QuoteIdent(createdDatabase))
	})
	databases, err := listDatabases(plugin.NewRequestContext(ctx, models.User{}, s, nil, nil, nil))
	if err != nil {
		t.Fatalf("list databases after create: %v", err)
	}
	if !pageHasName(databases.(plugin.Page[row]), createdDatabase) {
		t.Fatalf("created database was not listed: %#v", databases)
	}

	// View drop round-trip: create a view, drop it through the handler, verify gone.
	if _, err := s.pool.Exec(ctx, `CREATE VIEW public.shellcn_v AS SELECT 1 AS x`); err != nil {
		t.Fatalf("seed view: %v", err)
	}
	if _, err := dropView(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"schema": "public", "view": "shellcn_v"}, nil, nil)); err != nil {
		t.Fatalf("drop view: %v", err)
	}
	var vcount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_views WHERE schemaname='public' AND viewname='shellcn_v'`).Scan(&vcount); err != nil || vcount != 0 {
		t.Fatalf("expected view dropped, got %d err=%v", vcount, err)
	}

	// Schema create round-trip.
	defer func() { _, _ = s.pool.Exec(context.Background(), `DROP SCHEMA IF EXISTS shellcn_sc CASCADE`) }()
	if _, err := createSchema(rowMutationRC(ctx, s, nil, map[string]any{"name": "shellcn_sc"})); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	schemas, err := listSchemas(plugin.NewRequestContext(ctx, models.User{}, s, nil, nil, nil))
	if err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	if !pageHasName(schemas.(plugin.Page[row]), "shellcn_sc") {
		t.Fatalf("created schema was not listed: %#v", schemas)
	}

	if _, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS public.shellcn_people (
  id INT8 PRIMARY KEY,
  name STRING NOT NULL,
  access_token STRING NOT NULL
);
TRUNCATE public.shellcn_people;
INSERT INTO public.shellcn_people (id, name, access_token) VALUES (1, 'alice', 'secret-token')`); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = s.pool.Exec(cleanupCtx, `DROP TABLE IF EXISTS public.shellcn_people`)
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
	if len(page.Items) != 1 || page.Items[0]["access_token"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	result, err := executeQueryRequest(ctx, s, sqldb.QueryRequest{Query: `SELECT name, access_token FROM public.shellcn_people`})
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
	params := map[string]string{"schema": "public", "table": "shellcn_people"}
	if _, err := insertRow(rowMutationRC(ctx, s, params, map[string]any{"values": map[string]any{"id": 2, "name": "bob", "access_token": "tok"}})); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	var bobID int64
	if err := s.pool.QueryRow(ctx, `SELECT id FROM public.shellcn_people WHERE name = 'bob'`).Scan(&bobID); err != nil {
		t.Fatalf("read inserted row: %v", err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}, "values": map[string]any{"name": "bob2"}})); err != nil {
		t.Fatalf("update row: %v", err)
	}
	var name string
	if err := s.pool.QueryRow(ctx, `SELECT name FROM public.shellcn_people WHERE id = $1`, bobID).Scan(&name); err != nil || name != "bob2" {
		t.Fatalf("expected updated name bob2, got %q err=%v", name, err)
	}
	if _, err := deleteRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}})); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	var remaining int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM public.shellcn_people`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("expected 1 row after delete, got %d err=%v", remaining, err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"name": "alice"}, "values": map[string]any{"access_token": "x"}})); err == nil {
		t.Fatal("update with a non-primary-key key must be rejected")
	}

	// Column/index management via declarative DDL actions (create then drop the
	// index proves both work; CockroachDB drops indexes with table@index).
	if _, err := createIndex(rowMutationRC(ctx, s, params, map[string]any{"name": "ix_people_name", "columns": "name", "unique": false})); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if _, err := dropIndex(rowMutationRC(ctx, s, params, map[string]any{"name": "ix_people_name"})); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := dropColumn(rowMutationRC(ctx, s, params, map[string]any{"column": "access_token"})); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	var cols int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='shellcn_people' AND column_name='access_token'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("expected access_token column dropped, got %d err=%v", cols, err)
	}

	for name, fn := range map[string]func(*plugin.RequestContext) (any, error){
		"databases": listDatabases,
		"schemas":   listSchemas,
		"nodes":     listNodes,
		"jobs":      listJobs,
		"sessions":  listSessions,
		"queries":   listQueries,
		"ranges":    listRanges,
	} {
		if _, err := fn(rc); err != nil {
			t.Fatalf("%s route: %v", name, err)
		}
	}
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_COCKROACHDB_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_COCKROACHDB_DSN is not set")
	}
	name := "shellcn-cockroach-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--name", name,
		"-p", "127.0.0.1::26257",
		"cockroachdb/cockroach:latest",
		"start",
		"--insecure",
		"--store=type=mem,size=1GiB",
		"--listen-addr=0.0.0.0:26257",
		"--advertise-addr=localhost:26257",
		"--join=localhost:26257",
		"--http-addr=0.0.0.0:8080")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "26257/tcp")
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
		"database":  "defaultdb",
		"username":  "root",
		"password":  "",
		"tls_mode":  "disable",
		"read_only": false,
	}
	deadline := time.Now().Add(90 * time.Second)
	initialized := false
	var lastErr error
	for {
		if !initialized {
			if err := exec.CommandContext(ctx, "docker", "exec", name, "./cockroach", "init", "--insecure", "--host=localhost:26257").Run(); err == nil {
				initialized = true
			}
		}
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
			t.Fatalf("cockroach container did not become ready: %v\n%s", lastErr, out)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_COCKROACHDB_DSN: %v", err)
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
