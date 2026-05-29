package oracle

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

func TestOraclePluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_ORACLE_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_ORACLE_INTEGRATION=1 to run against Oracle Database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = false
	cfg["require_destructive_confirmation"] = true
	cfg["row_limit"] = 50
	cfg["query_timeout"] = "15s"

	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	seed(ctx, t, s)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, _ = s.db.ExecContext(cleanupCtx, `DROP USER SHELLCN_TEST CASCADE`)
	})

	list, err := listTables(plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, url.Values{"p.schema": {"SHELLCN_TEST"}}, nil))
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if !pageHasName(list.(plugin.Page[row]), "PEOPLE") {
		t.Fatalf("created table was not listed: %#v", list)
	}

	rows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"id": objectID("SHELLCN_TEST", "PEOPLE")}, nil, nil))
	if err != nil {
		t.Fatalf("table rows: %v", err)
	}
	page := rows.(plugin.Page[row])
	// The editable Data grid keeps Oracle's real (uppercase) column names so its
	// quoted UPDATE/DELETE identifiers match.
	if len(page.Items) != 1 || page.Items[0]["ACCESS_TOKEN"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	// Free-text search filters the data grid server-side (per-column).
	orPeople := map[string]string{"id": objectID("SHELLCN_TEST", "PEOPLE")}
	orMatch, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, orPeople, url.Values{"filter": {"alice"}}, nil))
	if err != nil {
		t.Fatalf("filtered rows: %v", err)
	}
	if len(orMatch.(plugin.Page[row]).Items) != 1 {
		t.Fatalf("filter 'alice' should match 1 row, got %#v", orMatch.(plugin.Page[row]).Items)
	}
	orMiss, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, orPeople, url.Values{"filter": {"zzz-nomatch"}}, nil))
	if err != nil {
		t.Fatalf("filtered rows (miss): %v", err)
	}
	if len(orMiss.(plugin.Page[row]).Items) != 0 {
		t.Fatalf("filter 'zzz-nomatch' should match 0 rows, got %#v", orMiss.(plugin.Page[row]).Items)
	}
	if key, ok := page.Items[0]["_key"].(map[string]any); !ok || key["ID"] == nil {
		t.Fatalf("table rows must carry a _key from the primary key: %#v", page.Items[0])
	}

	result, err := executeQueryRequest(ctx, s, "SHELLCN_TEST", sqldb.QueryRequest{Query: `SELECT NAME, ACCESS_TOKEN FROM PEOPLE`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][1] != sqldb.RedactedValue {
		t.Fatalf("expected redacted query result, got %#v", result.Rows)
	}

	params := map[string]string{"id": objectID("SHELLCN_TEST", "PEOPLE")}
	if _, err := insertRow(rowMutationRC(ctx, s, params, map[string]any{"values": map[string]any{"NAME": "bob", "ACCESS_TOKEN": "tok"}})); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	var bobID int64
	if err := s.db.QueryRowContext(ctx, `SELECT ID FROM SHELLCN_TEST.PEOPLE WHERE NAME = 'bob'`).Scan(&bobID); err != nil {
		t.Fatalf("read inserted row: %v", err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"ID": bobID}, "values": map[string]any{"NAME": "bob2"}})); err != nil {
		t.Fatalf("update row: %v", err)
	}
	var name string
	if err := s.db.QueryRowContext(ctx, `SELECT NAME FROM SHELLCN_TEST.PEOPLE WHERE ID = :1`, bobID).Scan(&name); err != nil || name != "bob2" {
		t.Fatalf("expected updated name bob2, got %q err=%v", name, err)
	}
	if _, err := deleteRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"ID": bobID}})); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	var remaining int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM SHELLCN_TEST.PEOPLE`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("expected 1 row after delete, got %d err=%v", remaining, err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"NAME": "alice"}, "values": map[string]any{"ACCESS_TOKEN": "x"}})); err == nil {
		t.Fatal("update with a non-primary-key key must be rejected")
	}

	// Column/index management via declarative DDL actions.
	if _, err := createIndex(rowMutationRC(ctx, s, params, map[string]any{"name": "IX_PEOPLE_NAME", "columns": "NAME", "unique": false})); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if _, err := dropIndex(rowMutationRC(ctx, s, map[string]string{"id": objectID("SHELLCN_TEST", "PEOPLE"), "name": "IX_PEOPLE_NAME"}, nil)); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := dropColumn(rowMutationRC(ctx, s, map[string]string{"id": objectID("SHELLCN_TEST", "PEOPLE"), "name": "ACCESS_TOKEN"}, nil)); err != nil {
		t.Fatalf("drop column: %v", err)
	}
	var cols int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM all_tab_columns WHERE owner = 'SHELLCN_TEST' AND table_name = 'PEOPLE' AND column_name = 'ACCESS_TOKEN'`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("expected ACCESS_TOKEN column dropped, got %d err=%v", cols, err)
	}

	// Foreign-key cells carry generic _links to the referenced table.
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE SHELLCN_TEST.ORDERS (ID NUMBER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY, PERSON_ID NUMBER REFERENCES SHELLCN_TEST.PEOPLE(ID))`); err != nil {
		t.Fatalf("create child table: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO SHELLCN_TEST.ORDERS (PERSON_ID) VALUES (1)`); err != nil {
		t.Fatalf("seed child row: %v", err)
	}
	orderRows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"id": objectID("SHELLCN_TEST", "ORDERS")}, nil, nil))
	if err != nil {
		t.Fatalf("child table rows: %v", err)
	}
	if links, ok := orderRows.(plugin.Page[row]).Items[0]["_links"].(map[string]plugin.ResourceRef); !ok || links["PERSON_ID"].UID != objectID("SHELLCN_TEST", "PEOPLE") {
		t.Fatalf("expected _links[PERSON_ID] -> PEOPLE, got %#v", orderRows.(plugin.Page[row]).Items[0]["_links"])
	}

	// Foreign-key relationship graph (ERD), owner-scoped on Oracle.
	graph, err := relationGraph(plugin.NewRequestContext(ctx, models.User{}, s, nil, url.Values{"p.schema": {"SHELLCN_TEST"}}, nil))
	if err != nil {
		t.Fatalf("relation graph: %v", err)
	}
	if !hasEdge(graph.(sqldb.GraphPayload), "SHELLCN_TEST.ORDERS", "SHELLCN_TEST.PEOPLE") {
		t.Fatalf("expected FK edge orders -> people, got %#v", graph)
	}
}

func rowMutationRC(ctx context.Context, s *Session, params map[string]string, body map[string]any) *plugin.RequestContext {
	raw, _ := json.Marshal(body)
	return plugin.NewRequestContext(ctx, models.User{ID: "u1"}, s, params, nil, raw)
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_ORACLE_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_ORACLE_DSN is not set")
	}
	name := "shellcn-oracle-it-" + time.Now().UTC().Format("20060102150405")
	password := "ShellCN123"
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "ORACLE_PASSWORD="+password,
		"-p", "127.0.0.1::1521",
		"gvenzl/oracle-free:slim-faststart")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "1521/tcp")
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
		"service":   "FREEPDB1",
		"username":  "SYSTEM",
		"password":  password,
		"tls_mode":  "disable",
		"read_only": false,
	}
	deadline := time.Now().Add(4 * time.Minute)
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
			t.Fatalf("oracle container did not become ready: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_ORACLE_DSN: %v", err)
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
		"service":   stringDefault(strings.TrimPrefix(u.Path, "/"), "FREEPDB1"),
		"username":  u.User.Username(),
		"password":  password,
		"tls_mode":  stringDefault(u.Query().Get("tls_mode"), "disable"),
		"read_only": false,
	}
}

func seed(ctx context.Context, t *testing.T, s *Session) {
	t.Helper()
	statements := []string{
		`BEGIN
  EXECUTE IMMEDIATE 'DROP USER SHELLCN_TEST CASCADE';
EXCEPTION
  WHEN OTHERS THEN
    IF SQLCODE != -1918 THEN RAISE; END IF;
END;`,
		`CREATE USER SHELLCN_TEST IDENTIFIED BY "ShellCN123"`,
		`GRANT CONNECT, RESOURCE TO SHELLCN_TEST`,
		`ALTER USER SHELLCN_TEST QUOTA UNLIMITED ON USERS`,
		`CREATE TABLE SHELLCN_TEST.PEOPLE (
  ID NUMBER GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  NAME VARCHAR2(255) NOT NULL,
  ACCESS_TOKEN VARCHAR2(255) NOT NULL
)`,
		`INSERT INTO SHELLCN_TEST.PEOPLE (NAME, ACCESS_TOKEN) VALUES ('alice', 'secret-token')`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed database: %v", err)
		}
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
