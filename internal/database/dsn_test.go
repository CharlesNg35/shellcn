package database

import (
	"strings"
	"testing"
)

func TestBuildPostgresDSNDefaults(t *testing.T) {
	dsn, err := buildPostgresDSN(Config{
		User: "shellcn",
		Name: "shellcn",
	})
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}

	expected := "host=localhost port=5432 user=shellcn dbname=shellcn sslmode=disable"
	if dsn != expected {
		t.Fatalf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildPostgresDSNWithOptions(t *testing.T) {
	dsn, err := buildPostgresDSN(Config{
		User:     "user",
		Name:     "db",
		Host:     "db.example.com",
		Port:     6543,
		Password: "pass",
		Options: map[string]string{
			"sslmode":     "require",
			"search_path": "public",
		},
	})
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}

	if !containsAll(
		dsn,
		"host=db.example.com",
		"port=6543",
		"user=user",
		"dbname=db",
		"password=pass",
		"sslmode=require",
		"search_path=public",
	) {
		t.Fatalf("dsn missing expected components: %q", dsn)
	}
}

func TestBuildPostgresDSNRequiresUserAndName(t *testing.T) {
	if _, err := buildPostgresDSN(Config{}); err == nil {
		t.Fatalf("expected error for missing credentials")
	}
}

func TestBuildMySQLDSNDefaults(t *testing.T) {
	dsn, err := buildMySQLDSN(Config{
		User: "shellcn",
		Name: "shellcn",
	})
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}

	expected := "shellcn@tcp(127.0.0.1:3306)/shellcn?charset=utf8mb4&loc=Local&parseTime=True"
	if dsn != expected {
		t.Fatalf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildMySQLDSNWithOptions(t *testing.T) {
	dsn, err := buildMySQLDSN(Config{
		User:     "user",
		Password: "secret",
		Name:     "db",
		Host:     "db.example.com",
		Port:     3307,
		Options: map[string]string{
			"tls": "skip-verify",
		},
	})
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}

	if !containsAll(
		dsn,
		"user:secret@tcp(db.example.com:3307)/db?",
		"charset=utf8mb4",
		"loc=Local",
		"parseTime=True",
		"tls=skip-verify",
	) {
		t.Fatalf("dsn missing expected components: %q", dsn)
	}
}

func TestBuildMySQLDSNRequiresUserAndName(t *testing.T) {
	if _, err := buildMySQLDSN(Config{Host: "localhost"}); err == nil {
		t.Fatalf("expected error for missing credentials")
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
