package mssql

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

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestMSSQLPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_MSSQL_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_MSSQL_INTEGRATION=1 to run against Microsoft SQL Server")
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

	seed(ctx, t, s)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = s.db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS [shellcn].[dbo].[people]`)
		_, _ = s.db.ExecContext(cleanupCtx, `DROP DATABASE [shellcn]`)
	})

	list, err := listTables(plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, url.Values{"p.database": {"shellcn"}}, nil))
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if !pageHasName(list.(plugin.Page[row]), "people") {
		t.Fatalf("created table was not listed: %#v", list)
	}

	rows, err := tableRows(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"id": objectID("shellcn", "dbo", "people")}, nil, nil))
	if err != nil {
		t.Fatalf("table rows: %v", err)
	}
	page := rows.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["access_token"] != sqldb.RedactedValue {
		t.Fatalf("expected redacted table data, got %#v", page.Items)
	}

	result, err := executeQueryRequest(ctx, s, "shellcn", sqldb.QueryRequest{Query: `SELECT name, access_token FROM dbo.people`})
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
	params := map[string]string{"id": objectID("shellcn", "dbo", "people")}
	if _, err := insertRow(rowMutationRC(ctx, s, params, map[string]any{"values": map[string]any{"name": "bob", "access_token": "tok"}})); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	var bobID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM [shellcn].[dbo].[people] WHERE name = N'bob'`).Scan(&bobID); err != nil {
		t.Fatalf("read inserted row: %v", err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}, "values": map[string]any{"name": "bob2"}})); err != nil {
		t.Fatalf("update row: %v", err)
	}
	var name string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM [shellcn].[dbo].[people] WHERE id = @p1`, bobID).Scan(&name); err != nil || name != "bob2" {
		t.Fatalf("expected updated name bob2, got %q err=%v", name, err)
	}
	if _, err := deleteRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"id": bobID}})); err != nil {
		t.Fatalf("delete row: %v", err)
	}
	var remaining int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM [shellcn].[dbo].[people]`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("expected 1 row after delete, got %d err=%v", remaining, err)
	}
	if _, err := updateRow(rowMutationRC(ctx, s, params, map[string]any{"key": map[string]any{"name": "alice"}, "values": map[string]any{"access_token": "x"}})); err == nil {
		t.Fatal("update with a non-primary-key key must be rejected")
	}
}

func rowMutationRC(ctx context.Context, s *Session, params map[string]string, body map[string]any) *plugin.RequestContext {
	raw, _ := json.Marshal(body)
	return plugin.NewRequestContext(ctx, models.User{ID: "u1"}, s, params, nil, raw)
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if raw := os.Getenv("SHELLCN_MSSQL_DSN"); raw != "" {
		return configFromDSN(t, raw)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_MSSQL_DSN is not set")
	}
	name := "shellcn-mssql-it-" + time.Now().UTC().Format("20060102150405")
	password := "ShellCN!23456"
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-e", "ACCEPT_EULA=Y",
		"-e", "MSSQL_SA_PASSWORD="+password,
		"-p", "127.0.0.1::1433",
		"mcr.microsoft.com/mssql/server:2022-latest")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "1433/tcp")
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
		"database":  "master",
		"username":  "sa",
		"password":  password,
		"encrypt":   "require",
		"read_only": false,
	}
	deadline := time.Now().Add(90 * time.Second)
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
			t.Fatalf("mssql container did not become ready: %v", err)
		}
		time.Sleep(time.Second)
	}
}

func configFromDSN(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse SHELLCN_MSSQL_DSN: %v", err)
	}
	port := defaultPort
	if u.Port() != "" {
		if port, err = strconv.Atoi(u.Port()); err != nil {
			t.Fatalf("parse DSN port: %v", err)
		}
	}
	password, _ := u.User.Password()
	encrypt := stringDefault(u.Query().Get("encrypt"), "require")
	if trust := u.Query().Get("TrustServerCertificate"); strings.EqualFold(trust, "true") {
		encrypt = "require"
	}
	return map[string]any{
		"host":      u.Hostname(),
		"port":      port,
		"database":  stringDefault(strings.TrimPrefix(u.Path, "/"), "master"),
		"username":  u.User.Username(),
		"password":  password,
		"encrypt":   encrypt,
		"read_only": false,
	}
}

func seed(ctx context.Context, t *testing.T, s *Session) {
	t.Helper()
	statements := []string{
		`IF DB_ID(N'shellcn') IS NULL CREATE DATABASE [shellcn]`,
		`DROP TABLE IF EXISTS [shellcn].[dbo].[people]`,
		`CREATE TABLE [shellcn].[dbo].[people] (
  id bigint IDENTITY(1,1) PRIMARY KEY,
  name nvarchar(255) NOT NULL,
  access_token nvarchar(255) NOT NULL
)`,
		`INSERT INTO [shellcn].[dbo].[people] (name, access_token) VALUES (N'alice', N'secret-token')`,
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
