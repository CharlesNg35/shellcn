package mongodb

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/transport"
	"github.com/charlesng/shellcn/plugins/shared/sqldb"
)

func TestMongoDBPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_MONGODB_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_MONGODB_INTEGRATION=1 to run against MongoDB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	coll := s.client.Database("shellcn").Collection("people")
	_ = coll.Drop(ctx)
	if _, err := coll.InsertOne(ctx, bson.M{"_id": "ada", "name": "Ada", "role": "admin"}); err != nil {
		t.Fatalf("seed MongoDB: %v", err)
	}

	rc := plugin.NewRequestContext(ctx, models.User{ID: "u1", Username: "admin"}, s, nil, nil, nil)
	databases, err := listDatabases(rc)
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if !pageHasName(databases.(plugin.Page[row]), "shellcn") {
		t.Fatalf("shellcn database missing: %#v", databases)
	}

	docs, err := listDocuments(plugin.NewRequestContext(ctx, models.User{}, s, map[string]string{"database": "shellcn", "collection": "people"}, nil, nil))
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	page := docs.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["name"] != "Ada" {
		t.Fatalf("unexpected documents: %#v", page.Items)
	}
	result, err := executeCommandRequest(ctx, s, "shellcn", sqldb.QueryRequest{Query: `{"find":"people","filter":{"_id":"ada"},"limit":1}`})
	if err != nil {
		t.Fatalf("command: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("unexpected command result: %#v", result.Rows)
	}
}

func integrationConfig(ctx context.Context, t *testing.T) map[string]any {
	t.Helper()
	if addr := os.Getenv("SHELLCN_MONGODB_ADDR"); addr != "" {
		host, portText, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatalf("parse SHELLCN_MONGODB_ADDR: %v", err)
		}
		port, err := strconv.Atoi(portText)
		if err != nil {
			t.Fatalf("parse MongoDB port: %v", err)
		}
		return map[string]any{"host": host, "port": port, "database": "admin", "tls_mode": "disable", "read_only": false}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_MONGODB_ADDR is not set")
	}
	name := "shellcn-mongodb-it-" + time.Now().UTC().Format("20060102150405")
	run(ctx, t, "docker", "run", "-d", "--rm", "--name", name, "-p", "127.0.0.1::27017", "mongo:7")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).Run()
	})
	out := run(ctx, t, "docker", "port", name, "27017/tcp")
	host, portText, err := net.SplitHostPort(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse docker port %q: %v", portText, err)
	}
	cfg := map[string]any{"host": host, "port": port, "database": "admin", "tls_mode": "disable", "read_only": false}
	deadline := time.Now().Add(35 * time.Second)
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
			t.Fatalf("MongoDB container did not become ready: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
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
