package smb

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestParseOptionsValidatesAuthFields(t *testing.T) {
	for name, cfg := range map[string]map[string]any{
		"password missing password": {"host": "smb.example.com", "share": "files", "auth": "password", "username": "alice"},
		"credential missing secret": {
			"host": "smb.example.com", "share": "files", "auth": "credential", plugin.CredentialIdentity: "alice",
		},
		"unsupported auth": {"host": "smb.example.com", "share": "files", "auth": "token", "username": "alice", "password": "pw"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOptions(plugin.ConnectConfig{Config: cfg}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	t.Run("guest", func(t *testing.T) {
		opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{"host": "smb.example.com", "share": "files", "auth": "guest"}})
		if err != nil {
			t.Fatalf("guest auth should validate: %v", err)
		}
		if opts.Username != "" || opts.Password != "" {
			t.Fatalf("guest should not carry user material: %+v", opts)
		}
	})

	t.Run("credential", func(t *testing.T) {
		opts, err := parseOptions(plugin.ConnectConfig{Config: map[string]any{
			"host": "smb.example.com", "share": "files", "auth": "credential",
			plugin.CredentialIdentity: "alice", plugin.CredentialSecret: "pw",
		}})
		if err != nil {
			t.Fatalf("credential auth should validate: %v", err)
		}
		if opts.Username != "alice" || opts.Password != "pw" {
			t.Fatalf("credential material not applied: %+v", opts)
		}
	})
}

func TestNormalizeRootPath(t *testing.T) {
	for name, tc := range map[string]struct {
		raw  string
		want string
	}{
		"empty":        {raw: "", want: "/"},
		"dot":          {raw: ".", want: "/"},
		"slash":        {raw: "/", want: "/"},
		"backslash":    {raw: `\`, want: "/"},
		"nested slash": {raw: "projects/reports", want: "/projects/reports"},
		"nested smb":   {raw: `projects\reports`, want: "/projects/reports"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := normalizeRootPath(tc.raw); got != tc.want {
				t.Fatalf("normalizeRootPath(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestSMBPathRoot(t *testing.T) {
	for _, raw := range []string{"/", `\`} {
		if got := smbPath(normalizeRootPath(raw)); got != "." {
			t.Fatalf("smbPath(normalizeRootPath(%q)) = %q, want .", raw, got)
		}
	}
}
