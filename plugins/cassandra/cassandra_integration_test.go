package cassandra

import (
	"context"
	"encoding/json"
	"net"
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

func TestCassandraPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_CASSANDRA_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_CASSANDRA_INTEGRATION=1 to run against Cassandra")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = false
	cfg["require_destructive_confirmation"] = true
	cfg["row_limit"] = 50
	cfg["page_size"] = 50
	cfg["query_timeout"] = "20s"
	cfg["consistency"] = "LOCAL_ONE"

	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	rc := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, mustJSON(t, map[string]any{
		"name":               "shellcn_it",
		"replication_class":  "SimpleStrategy",
		"replication_factor": 1,
		"durable_writes":     true,
		"if_not_exists":      true,
	}))
	if _, err := createKeyspace(rc); err != nil {
		t.Fatalf("create keyspace route: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_ = execCQL(cleanupCtx, s, `DROP KEYSPACE IF EXISTS "shellcn_it"`)
	})

	createTableRC := plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"keyspace": "shellcn_it"}, nil, mustJSON(t, map[string]any{
		"name": "people",
		"columns": []map[string]any{
			{"name": "id", "type": "uuid"},
			{"name": "name", "type": "text"},
			{"name": "access_token", "type": "text"},
		},
		"primary_key":   "id",
		"if_not_exists": true,
	}))
	if _, err := createTable(createTableRC); err != nil {
		t.Fatalf("create table route: %v", err)
	}
	if err := execCQL(ctx, s, `TRUNCATE "shellcn_it"."people"`); err != nil {
		t.Fatalf("truncate table: %v", err)
	}
	if err := execCQL(ctx, s, `INSERT INTO "shellcn_it"."people" (id, name, access_token) VALUES (11111111-1111-1111-1111-111111111111, 'alice', 'secret-token')`); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	baseRC := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	waitForTable(ctx, t, baseRC, "shellcn_it", "people")

	keyspaces, err := listKeyspaces(baseRC)
	if err != nil {
		t.Fatalf("list keyspaces: %v", err)
	}
	if !pageHasName(keyspaces.(plugin.Page[row]), "shellcn_it") {
		t.Fatalf("created keyspace was not listed: %#v", keyspaces)
	}

	tableRC := plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"keyspace": "shellcn_it", "table": "people"}, nil, nil)
	rows, err := tableRows(tableRC)
	if err != nil {
		t.Fatalf("table rows: %v", err)
	}
	page := rows.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["access_token"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	result, err := executeQueryRequest(ctx, s, sqldb.QueryRequest{Query: `SELECT name, access_token FROM "shellcn_it"."people"`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	tokenIndex := columnIndex(result.Columns, "access_token")
	if len(result.Rows) != 1 || tokenIndex < 0 || result.Rows[0][tokenIndex] != sqldb.RedactedValue {
		t.Fatalf("expected redacted query result, got columns=%#v rows=%#v", result.Columns, result.Rows)
	}

	for name, fn := range map[string]func(*plugin.RequestContext) (any, error){
		"columns":    tableColumnsRoute,
		"indexes":    tableIndexes,
		"definition": tableDefinition,
	} {
		if _, err := fn(tableRC); err != nil {
			t.Fatalf("table %s route: %v", name, err)
		}
	}
	for name, fn := range map[string]func(*plugin.RequestContext) (any, error){
		"views":      listViews,
		"types":      listTypes,
		"functions":  listFunctions,
		"nodes":      listNodes,
		"completion": completionRoute,
	} {
		if _, err := fn(baseRC); err != nil {
			t.Fatalf("%s route: %v", name, err)
		}
	}
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_CASSANDRA_ADDR"); raw != "" {
		host, portText, err := net.SplitHostPort(raw)
		if err != nil {
			t.Fatalf("parse SHELLCN_CASSANDRA_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse Cassandra port: %v", err)
		}
		return map[string]any{
			"hosts":     host,
			"port":      port,
			"auth":      authNone,
			"tls_mode":  "disable",
			"read_only": false,
		}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_CASSANDRA_ADDR is not set")
	}
	name := "shellcn-cassandra-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "CASSANDRA_CLUSTER_NAME=shellcn",
		"-e", "MAX_HEAP_SIZE=512M",
		"-e", "HEAP_NEWSIZE=128M",
		"-p", "127.0.0.1::9042",
		"cassandra:5")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "9042/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{
		"hosts":       host,
		"port":        port,
		"auth":        authNone,
		"tls_mode":    "disable",
		"read_only":   false,
		"consistency": "LOCAL_ONE",
	}
	deadline := time.Now().Add(3 * time.Minute)
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
			logs := exec.CommandContext(ctx, "docker", "logs", "--tail", "160", name)
			out, _ := logs.CombinedOutput()
			t.Fatalf("cassandra container did not become ready: %v\n%s", lastErr, out)
		}
		time.Sleep(2 * time.Second)
	}
}

func waitForTable(ctx context.Context, t *testing.T, rc *plugin.RequestContext, keyspace, table string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		list, err := listTables(rc)
		if err == nil && pageHasScopedName(list.(plugin.Page[row]), keyspace, table) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("table %s.%s was not listed: %v", keyspace, table, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("table %s.%s was not listed: %v", keyspace, table, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
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

func pageHasScopedName(page plugin.Page[row], namespace, name string) bool {
	for _, item := range page.Items {
		if item["keyspace"] == namespace && item["name"] == name {
			return true
		}
	}
	return false
}

func columnIndex(columns []string, column string) int {
	for i, name := range columns {
		if name == column {
			return i
		}
	}
	return -1
}
