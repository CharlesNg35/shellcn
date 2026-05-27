package webdav

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

func TestParseOptionsValidatesAuthFields(t *testing.T) {
	for name, cfg := range map[string]map[string]any{
		"password missing password": {"url": "https://dav.example.com/", "auth": "password", "username": "alice"},
		"credential missing secret": {
			"url": "https://dav.example.com/", "auth": "credential", service.CredentialIdentity: "alice",
		},
		"unsupported auth": {"url": "https://dav.example.com/", "auth": "token", "username": "alice", "password": "pw"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOptions(plugin.ConnectConfig{Config: cfg}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	t.Run("none", func(t *testing.T) {
		opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"url": "https://dav.example.com/", "auth": "none"}})
		if err != nil {
			t.Fatalf("none auth should validate: %v", err)
		}
		if opts.Username != "" || opts.Password != "" {
			t.Fatalf("none should not carry user material: %+v", opts)
		}
	})

	t.Run("credential", func(t *testing.T) {
		opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
			"url": "https://dav.example.com/", "auth": "credential",
			service.CredentialIdentity: "alice", service.CredentialSecret: "pw",
		}})
		if err != nil {
			t.Fatalf("credential auth should validate: %v", err)
		}
		if opts.Username != "alice" || opts.Password != "pw" {
			t.Fatalf("credential material not applied: %+v", opts)
		}
	})
}
