package ftpfs

import (
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestNormalizeOptionsValidatesAuthFields(t *testing.T) {
	for name, cfg := range map[string]map[string]any{
		"password missing password": {"host": "ftp.example.com", "auth": "password", "username": "alice"},
		"credential missing secret": {
			"host": "ftp.example.com", "auth": "credential", plugin.CredentialIdentity: "alice",
		},
		"unsupported auth": {"host": "ftp.example.com", "auth": "token", "username": "alice", "password": "pw"},
	} {
		t.Run(name, func(t *testing.T) {
			err := normalizeOptions(plugin.ConnectConfig{Config: cfg}, &Options{})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	t.Run("anonymous", func(t *testing.T) {
		cfg := plugin.ConnectConfig{Config: map[string]any{"host": "ftp.example.com", "auth": "anonymous"}}
		var opts Options
		if err := normalizeOptions(cfg, &opts); err != nil {
			t.Fatalf("anonymous auth should validate: %v", err)
		}
		if opts.Username != "anonymous" || !strings.Contains(opts.Password, "@") {
			t.Fatalf("anonymous credentials not normalized: %+v", opts)
		}
	})

	t.Run("credential", func(t *testing.T) {
		cfg := plugin.ConnectConfig{Config: map[string]any{
			"host": "ftp.example.com", "auth": "credential",
			plugin.CredentialIdentity: "alice", plugin.CredentialSecret: "pw",
		}}
		var opts Options
		if err := normalizeOptions(cfg, &opts); err != nil {
			t.Fatalf("credential auth should validate: %v", err)
		}
		if opts.Username != "alice" || opts.Password != "pw" {
			t.Fatalf("credential material not applied: %+v", opts)
		}
	})
}
