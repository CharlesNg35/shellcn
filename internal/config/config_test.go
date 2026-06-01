package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != ":8081" {
		t.Errorf("addr default: got %q", cfg.Server.Addr)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("log level default: got %q", cfg.Server.LogLevel)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != app.DefaultDatabaseDSN {
		t.Errorf("database defaults: got %+v", cfg.Database)
	}
	if cfg.Auth.SessionTTLDuration().String() != "24h0m0s" {
		t.Errorf("auth session TTL default: got %s", cfg.Auth.SessionTTLDuration())
	}
	if cfg.Bootstrap.AdminUsername != "admin" || cfg.Bootstrap.AdminPassword != "" {
		t.Errorf("bootstrap defaults: got %+v", cfg.Bootstrap)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("SHELLCN_SERVER_ADDR", ":9999")
	t.Setenv("SHELLCN_DATABASE_DRIVER", "postgres")
	t.Setenv("SHELLCN_AUTH_SESSION_TTL", "2h")
	t.Setenv("SHELLCN_BOOTSTRAP_ADMIN_USERNAME", "root")
	t.Setenv("SHELLCN_BOOTSTRAP_ADMIN_PASSWORD", "initial-secret")
	t.Setenv("SHELLCN_MASTER_KEY", "deadbeef")
	t.Setenv("SHELLCN_AI_KIND", "openrouter")
	t.Setenv("SHELLCN_AI_API_KEY", "sk-or")
	t.Setenv("SHELLCN_AI_MODEL", "openai/gpt-4o")

	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != ":9999" {
		t.Errorf("addr override: got %q", cfg.Server.Addr)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("driver override: got %q", cfg.Database.Driver)
	}
	if cfg.Secrets.MasterKey != "deadbeef" {
		t.Errorf("master key env: got %q", cfg.Secrets.MasterKey)
	}
	if cfg.Auth.SessionTTLDuration().String() != "2h0m0s" {
		t.Errorf("auth ttl env: got %s", cfg.Auth.SessionTTLDuration())
	}
	if cfg.Bootstrap.AdminUsername != "root" || cfg.Bootstrap.AdminPassword != "initial-secret" {
		t.Errorf("bootstrap env override: got %+v", cfg.Bootstrap)
	}
	if !cfg.AI.Configured() || cfg.AI.Model != "openai/gpt-4o" {
		t.Errorf("ai env override: got %+v", cfg.AI)
	}
}

func TestFileLoadWithEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	yaml := "server:\n  addr: \":7000\"\n  log_level: debug\nbootstrap:\n  admin_username: root\n  admin_password: file-secret\ndatabase:\n  driver: mysql\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Addr != ":7000" || cfg.Server.LogLevel != "debug" {
		t.Errorf("file values not applied: %+v", cfg.Server)
	}
	if cfg.Database.Driver != "mysql" {
		t.Errorf("file driver not applied: got %q", cfg.Database.Driver)
	}
	if cfg.Bootstrap.AdminUsername != "root" || cfg.Bootstrap.AdminPassword != "file-secret" {
		t.Errorf("file bootstrap values not applied: %+v", cfg.Bootstrap)
	}
	if cfg.SlogLevel().String() != "DEBUG" {
		t.Errorf("SlogLevel: got %s", cfg.SlogLevel())
	}

	// Env still overrides a file value.
	t.Setenv("SHELLCN_SERVER_ADDR", ":6000")
	cfg, err = config.Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg.Server.Addr != ":6000" {
		t.Errorf("env should override file: got %q", cfg.Server.Addr)
	}
}
