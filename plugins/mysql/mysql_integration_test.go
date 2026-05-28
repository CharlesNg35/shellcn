package mysql

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
	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestMySQLPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_MYSQL_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_MYSQL_INTEGRATION=1 to run against MySQL or MariaDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	var vcount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", createdDatabase, "shellcn_v").Scan(&vcount); err != nil || vcount != 0 {
		t.Fatalf("expected view dropped, got %d err=%v", vcount, err)
	}

	seedStatements := []string{
		`
CREATE TABLE IF NOT EXISTS shellcn_people (
  id bigint unsigned auto_increment PRIMARY KEY,
  name varchar(255) NOT NULL,
  access_token varchar(255) NOT NULL
);`,
		`TRUNCATE TABLE shellcn_people`,
		`INSERT INTO shellcn_people (name, access_token) VALUES ('alice', 'secret-token')`,
	}
	for _, statement := range seedStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed database: %v", err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
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

	result, err := executeQueryRequest(ctx, s, sqldb.QueryRequest{Query: `SELECT name, access_token FROM shellcn_people`})
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

	database := cfg["database"].(string)
	params := map[string]string{"database": database, "table": "shellcn_people"}
	if _, err := insertRow(rowMutationRC(ctx, s, params, map[string]any{"values": map[string]any{"name": "bob", "access_token": "tok"}})); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	var bobID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM shellcn_people WHERE name = 'bob'`).Scan(&bobID); err != nil {
		t.Fatalf("read inserted row: %v", err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}, "values": map[string]any{"name": "bob2"}})); err != nil {
		t.Fatalf("update row: %v", err)
	}
	var name string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM shellcn_people WHERE id = ?`, bobID).Scan(&name); err != nil || name != "bob2" {
		t.Fatalf("expected updated name bob2, got %q err=%v", name, err)
	}
	if _, err := deleteRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}})); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	var remaining int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shellcn_people`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("expected 1 row after delete, got %d err=%v", remaining, err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"name": "alice"}, "values": map[string]any{"access_token": "x"}})); err == nil {
		t.Fatal("update with a non-primary-key key must be rejected")
	}

	// Hierarchical tree: databases are expandable branches, drilling into tables.
	dbTree, err := treeDatabases(plugin.NewRequestContext(ctx, models.User{}, s, nil, nil, nil))
	if err != nil {
		t.Fatalf("tree databases: %v", err)
	}
	var branch *plugin.TreeNode
	for i, n := range dbTree.(plugin.Page[plugin.TreeNode]).Items {
		if n.Label == database {
			branch = &dbTree.(plugin.Page[plugin.TreeNode]).Items[i]
		}
	}
	if branch == nil || branch.ChildrenSource == nil || branch.Leaf {
		t.Fatalf("database node must be an expandable branch: %#v", branch)
	}
	relTree, err := treeRelations(plugin.NewRequestContext(ctx, models.User{}, s, nil, url.Values{"p.database": {database}}, nil))
	if err != nil {
		t.Fatalf("tree relations: %v", err)
	}
	found := false
	for _, n := range relTree.(plugin.Page[plugin.TreeNode]).Items {
		if n.Label == "shellcn_people" && n.Leaf {
			found = true
		}
	}
	if !found {
		t.Fatalf("table leaf not found under database branch: %#v", relTree)
	}

	// Column/index management via declarative DDL actions.
	if _, err := createIndex(rowMutationRC(ctx, s, params, map[string]any{"name": "ix_people_name", "columns": "name", "unique": false})); err != nil {
		t.Fatalf("create index: %v", err)
	}
	var idx int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = ? AND table_name = 'shellcn_people' AND index_name = 'ix_people_name'`, database).Scan(&idx); err != nil || idx == 0 {
		t.Fatalf("expected created index, got %d err=%v", idx, err)
	}
	if _, err := dropIndex(rowMutationRC(ctx, s, map[string]string{"database": database, "table": "shellcn_people", "name": "ix_people_name"}, nil)); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := dropColumn(rowMutationRC(ctx, s, map[string]string{"database": database, "table": "shellcn_people", "name": "access_token"}, nil)); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	var cols int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = 'shellcn_people' AND column_name = 'access_token'`, database).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("expected access_token column dropped, got %d err=%v", cols, err)
	}

	// Foreign-key cells carry generic _links to the referenced table.
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE shellcn_orders (id bigint unsigned AUTO_INCREMENT PRIMARY KEY, person_id bigint unsigned, FOREIGN KEY (person_id) REFERENCES shellcn_people(id))`); err != nil {
		t.Fatalf("create child table: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO shellcn_orders (person_id) VALUES (1)`); err != nil {
		t.Fatalf("seed child row: %v", err)
	}
	orderRows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": database, "table": "shellcn_orders"}, nil, nil))
	if err != nil {
		t.Fatalf("child table rows: %v", err)
	}
	if links, ok := orderRows.(plugin.Page[row]).Items[0]["_links"].(map[string]plugin.ResourceRef); !ok || links["person_id"].Name != "shellcn_people" {
		t.Fatalf("expected _links[person_id] -> shellcn_people, got %#v", orderRows.(plugin.Page[row]).Items[0]["_links"])
	}

	// Foreign-key relationship graph (ERD) over the FK created above.
	graph, err := relationGraph(plugin.NewRequestContext(ctx, models.User{}, s, nil, url.Values{"p.database": {database}}, nil))
	if err != nil {
		t.Fatalf("relation graph: %v", err)
	}
	if !hasEdge(graph.(sqldb.GraphPayload), database+".shellcn_orders", database+".shellcn_people") {
		t.Fatalf("expected FK edge orders -> people, got %#v", graph)
	}
}

func rowMutationRC(ctx context.Context, s *Session, params map[string]string, body map[string]any) *plugin.RequestContext {
	raw, _ := json.Marshal(body)
	return plugin.NewRequestContext(ctx, models.User{ID: "u1"}, s, params, nil, raw)
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_MYSQL_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_MYSQL_DSN is not set")
	}
	name := "shellcn-mysql-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "MYSQL_ROOT_PASSWORD=shellcn",
		"-e", "MYSQL_DATABASE=shellcn",
		"-p", "127.0.0.1::3306",
		"mysql:8.4")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "3306/tcp")
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
		"username":  "root",
		"password":  "shellcn",
		"tls_mode":  "disable",
		"read_only": false,
	}
	deadline := time.Now().Add(45 * time.Second)
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
			t.Fatalf("mysql container did not become ready: %v", err)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	if !strings.Contains(raw, "://") {
		cfg, err := mysqldriver.ParseDSN(raw)
		if err != nil {
			t.Fatalf("parse SHELLCN_MYSQL_DSN: %v", err)
		}
		host, portText, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			t.Fatalf("parse DSN address %q: %v", cfg.Addr, err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse DSN port: %v", err)
		}
		return map[string]any{
			"host":      host,
			"port":      port,
			"database":  cfg.DBName,
			"username":  cfg.User,
			"password":  cfg.Passwd,
			"tls_mode":  stringDefault(cfg.TLSConfig, "disable"),
			"read_only": false,
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_MYSQL_DSN: %v", err)
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
		"tls_mode":  stringDefault(u.Query().Get("tls"), "disable"),
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

func hasEdge(g sqldb.GraphPayload, source, target string) bool {
	for _, e := range g.Edges {
		if e.Source == source && e.Target == target {
			return true
		}
	}
	return false
}
