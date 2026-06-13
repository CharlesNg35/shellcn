package rdp

import (
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestParseConnectOptionsRejectsURLHostAndUnknownAuth(t *testing.T) {
	base := map[string]any{"host": "https://rdp.example", "username": "u", "password": "p"}
	if _, err := parseConnectOptions(plugin.ConnectConfig{Config: base}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("URL host should fail as invalid input, got %v", err)
	}
	base["host"] = "rdp.example"
	base["auth"] = "kerberos"
	if _, err := parseConnectOptions(plugin.ConnectConfig{Config: base}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unknown auth should fail as invalid input, got %v", err)
	}
}

func TestParseConnectOptionsRequiresPasswordMaterial(t *testing.T) {
	_, err := parseConnectOptions(plugin.ConnectConfig{Config: map[string]any{
		"host": "rdp.example", "username": "u", "password": "",
	}})
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("missing password should fail as invalid input, got %v", err)
	}
}

func TestManifestUsesDesktopSizeSelect(t *testing.T) {
	var found bool
	for _, group := range New().Manifest().Config.Groups {
		for _, field := range group.Fields {
			if field.Key != "resolution" {
				continue
			}
			found = true
			if field.Label != "Desktop size" || field.Type != plugin.FieldSelect {
				t.Fatalf("resolution field = %#v", field)
			}
			if len(field.Options) < 4 {
				t.Fatalf("resolution should provide common desktop sizes: %#v", field.Options)
			}
		}
	}
	if !found {
		t.Fatal("missing resolution field")
	}
}
