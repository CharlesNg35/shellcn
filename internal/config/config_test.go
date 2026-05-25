package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charlesng/shellcn/internal/config"
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
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != "shellcn.db" {
		t.Errorf("database defaults: got %+v", cfg.Database)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("SHELLCN_SERVER_ADDR", ":9999")
	t.Setenv("SHELLCN_DATABASE_DRIVER", "postgres")
	t.Setenv("SHELLCN_MASTER_KEY", "deadbeef") // bound to the historical name

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
		t.Errorf("master key env (legacy name): got %q", cfg.Secrets.MasterKey)
	}
}

func TestFileLoadWithEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	yaml := "server:\n  addr: \":7000\"\n  log_level: debug\ndatabase:\n  driver: mysql\n"
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
