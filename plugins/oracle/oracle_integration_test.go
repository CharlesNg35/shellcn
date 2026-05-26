package oracle

import (
	"context"
	"net"
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
	if len(page.Items) != 1 || page.Items[0]["access_token"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	result, err := executeQueryRequest(ctx, s, "SHELLCN_TEST", sqldb.QueryRequest{Query: `SELECT NAME, ACCESS_TOKEN FROM PEOPLE`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][1] != sqldb.RedactedValue {
		t.Fatalf("expected redacted query result, got %#v", result.Rows)
	}
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
