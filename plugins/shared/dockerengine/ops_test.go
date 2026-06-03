package dockerengine

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/registry"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestNormalizeKillSignal(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "SIGKILL", false},
		{"sigterm", "SIGTERM", false},
		{"TERM", "SIGTERM", false},
		{" int ", "SIGINT", false},
		{"SIGUSR1", "SIGUSR1", false},
		{"SIGFOO", "", true},
		{"rm -rf", "", true},
	}
	for _, c := range cases {
		got, err := normalizeKillSignal(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("normalizeKillSignal(%q) expected error, got %q", c.in, got)
			}
			if !errors.Is(err, plugin.ErrInvalidInput) {
				t.Fatalf("normalizeKillSignal(%q) error = %v, want ErrInvalidInput", c.in, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeKillSignal(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("normalizeKillSignal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateResourceName(t *testing.T) {
	for _, ok := range []string{"web", "my-app_1", "a.b.c"} {
		if err := validateResourceName(ok); err != nil {
			t.Fatalf("validateResourceName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "  ", "has space", "with/slash", "tab\tname"} {
		if err := validateResourceName(bad); err == nil {
			t.Fatalf("validateResourceName(%q) = nil, want error", bad)
		} else if !errors.Is(err, plugin.ErrInvalidInput) {
			t.Fatalf("validateResourceName(%q) error = %v, want ErrInvalidInput", bad, err)
		}
	}
}

func TestEncodeRegistryAuth(t *testing.T) {
	got, err := encodeRegistryAuth("", "")
	if err != nil || got != "" {
		t.Fatalf("encodeRegistryAuth(empty) = (%q,%v), want (\"\",nil)", got, err)
	}

	got, err = encodeRegistryAuth("robot", "secret")
	if err != nil {
		t.Fatalf("encodeRegistryAuth: %v", err)
	}
	raw, err := base64.URLEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode auth: %v", err)
	}
	var cfg registry.AuthConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal auth: %v", err)
	}
	if cfg.Username != "robot" || cfg.Password != "secret" {
		t.Fatalf("auth = %+v, want robot/secret", cfg)
	}
}

func TestTarDockerfile(t *testing.T) {
	r, err := tarDockerfile("Dockerfile", "FROM alpine:3.20\n")
	if err != nil {
		t.Fatalf("tarDockerfile: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read tar: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("tar build context is empty")
	}
	// The Dockerfile name must be embedded in the tar header block.
	if !strings.Contains(string(data), "Dockerfile") {
		t.Fatal("tar does not name the Dockerfile entry")
	}
}

func TestCollectBuildOutputSurfacesError(t *testing.T) {
	stream := `{"stream":"Step 1/1 : FROM alpine\n"}` + "\n" + `{"error":"pull access denied"}` + "\n"
	out, err := collectBuildOutput(strings.NewReader(stream))
	if err == nil {
		t.Fatal("expected build error, got nil")
	}
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(out, "Step 1/1") {
		t.Fatalf("output should retain pre-error log, got %q", out)
	}
}

func TestCollectBuildOutputSuccess(t *testing.T) {
	stream := `{"stream":"Step 1/2 : FROM alpine\n"}` + "\n" + `{"stream":"Successfully built abc123\n"}` + "\n"
	out, err := collectBuildOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("collectBuildOutput: %v", err)
	}
	if !strings.Contains(out, "Successfully built") {
		t.Fatalf("output = %q, want success line", out)
	}
}
