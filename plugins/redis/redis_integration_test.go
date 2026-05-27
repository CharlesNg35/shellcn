package redis

import (
	"context"
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

func TestRedisPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_REDIS_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_REDIS_INTEGRATION=1 to run against Redis")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cfg := integrationConfig(ctx, t)
	cfg["read_only"] = false
	cfg["require_write_confirmation"] = true

	sess, err := connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    transport.NewDirectForConnection(models.Connection{Config: cfg}),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	s := sess.(*Session)

	if err := s.client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush db: %v", err)
	}
	if err := s.client.Set(ctx, "shellcn:string", "hello", 0).Err(); err != nil {
		t.Fatalf("seed string: %v", err)
	}
	if err := s.client.HSet(ctx, "shellcn:hash", map[string]string{"name": "ada"}).Err(); err != nil {
		t.Fatalf("seed hash: %v", err)
	}

	rc := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	list, err := listKeys(rc)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if !hasKey(list.(plugin.Page[keyEntry]), "shellcn:string") {
		t.Fatalf("seeded string key missing: %#v", list)
	}
	detail, err := readKey(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"key": "shellcn:hash"}, nil, nil))
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	hash := detail.(keyDetail)
	if hash.Type != "hash" {
		t.Fatalf("expected hash detail, got %#v", hash)
	}
	result, err := executeCommandRequest(ctx, s, sqldb.QueryRequest{Query: "GET shellcn:string"})
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "hello" {
		t.Fatalf("unexpected command result: %#v", result.Rows)
	}
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if addr := os.Getenv("SHELLCN_REDIS_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_REDIS_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse Redis port: %v", err)
		}
		return map[string]any{"host": host, "port": port, "database": 0, "tls_mode": "disable", "read_only": false}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_REDIS_ADDR is not set")
	}
	name := "shellcn-redis-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name, "-p", "127.0.0.1::6379", "redis:7-alpine")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "6379/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{"host": host, "port": port, "database": 0, "tls_mode": "disable", "read_only": false}
	deadline := time.Now().Add(20 * time.Second)
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
			t.Fatalf("Redis container did not become ready: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
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

func hasKey(page plugin.Page[keyEntry], key string) bool {
	for _, item := range page.Items {
		if item.Key == key {
			return true
		}
	}
	return false
}
