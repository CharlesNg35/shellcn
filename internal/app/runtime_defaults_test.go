package app

import (
	"strings"
	"testing"
)

func TestApplyRuntimeDefaultsGeneratesMissingSecrets(t *testing.T) {
	cfg := &Config{}

	generated, err := ApplyRuntimeDefaults(cfg)
	if err != nil {
		t.Fatalf("ApplyRuntimeDefaults returned error: %v", err)
	}

	if cfg.Auth.JWT.Secret == "" {
		t.Fatal("expected JWT secret to be generated")
	}
	if cfg.Vault.EncryptionKey == "" {
		t.Fatal("expected vault encryption key to be generated")
	}
	if !generated["auth.jwt.secret"] || !generated["vault.encryption_key"] {
		t.Fatalf("expected generated map to include both keys: %#v", generated)
	}
}

func TestApplyRuntimeDefaultsPreservesExistingSecrets(t *testing.T) {
	cfg := &Config{}
	cfg.Auth.JWT.Secret = strings.Repeat("a", 10)
	cfg.Vault.EncryptionKey = strings.Repeat("b", 10)

	generated, err := ApplyRuntimeDefaults(cfg)
	if err != nil {
		t.Fatalf("ApplyRuntimeDefaults returned error: %v", err)
	}

	if len(generated) != 0 {
		t.Fatalf("expected no keys generated, got %#v", generated)
	}
}

func TestApplyRuntimeDefaultsNilConfig(t *testing.T) {
	_, err := ApplyRuntimeDefaults(nil)
	if err == nil || !strings.Contains(err.Error(), "config is nil") {
		t.Fatalf("expected nil config error, got %v", err)
	}
}

func TestGenerateHexKey(t *testing.T) {
	key, err := generateHexKey(4)
	if err != nil {
		t.Fatalf("generateHexKey returned error: %v", err)
	}
	if len(key) != 8 {
		t.Fatalf("expected encoded length 8, got %d", len(key))
	}

	if _, err = generateHexKey(0); err == nil {
		t.Fatal("expected error when length <= 0")
	}
}
