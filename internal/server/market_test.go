package server_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/internal/pluginmarket"
	"github.com/charlesng35/shellcn/internal/server"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
)

// marketFixture builds the demo plugin, serves it plus an index over httptest,
// and returns the harness with the marketplace wired.
func marketFixture(t *testing.T) (*harness, string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "demo")
	build := exec.Command("go", "build", "-o", bin,
		"github.com/charlesng35/shellcn/internal/extplugin/testdata/demoplugin")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build demo: %v\n%s", err, out)
	}
	payload, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)

	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(assets.Close)

	idx := pluginmarket.Index{
		SchemaVersion: 1,
		Plugins: []pluginmarket.Entry{{
			Name: "demo", DisplayName: "Demo", Description: "test plugin",
			Repo: "github.com/acme/demo", License: "MIT", Maintainers: []string{"acme"},
			Versions: []pluginmarket.Version{{
				Version: "0.9.0", SDK: "v0.1.3", APIVersion: 1, ProtocolVersion: grpcplugin.ProtocolVersion,
				Assets: map[string]pluginmarket.Asset{
					runtime.GOOS + "/" + runtime.GOARCH: {
						SHA256: hex.EncodeToString(sum[:]),
						URLs:   []string{assets.URL + "/demo"},
					},
				},
			}},
		}},
	}
	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(idx)
	}))
	t.Cleanup(index.Close)

	pluginsDir := t.TempDir()
	mgr := extplugin.NewManager(pluginsDir)
	t.Cleanup(mgr.Close)

	h := newHarness(t, func(d *server.Deps) {
		d.ExtPlugins = mgr
		d.PluginsDir = pluginsDir
		d.Market = pluginmarket.New([]string{index.URL})
	})
	return h, pluginsDir
}

func TestMarketListAndInstall(t *testing.T) {
	h, pluginsDir := marketFixture(t)

	if resp := h.do(t, http.MethodGet, "/api/admin/market", "op", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("non-admin must be forbidden, got %d", resp.Status)
	}

	resp := h.do(t, http.MethodGet, "/api/admin/market", "admin", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("list: %d %s", resp.Status, resp.Body)
	}
	var list struct {
		Enabled bool `json:"enabled"`
		Plugins []struct {
			Name       string `json:"name"`
			Compatible bool   `json:"compatible"`
			Managed    bool   `json:"managed"`
			Latest     *struct {
				Version string `json:"version"`
			} `json:"latest"`
		} `json:"plugins"`
	}
	mustDecode(t, resp.Body, &list)
	if !list.Enabled || len(list.Plugins) != 1 || !list.Plugins[0].Compatible || list.Plugins[0].Managed {
		t.Fatalf("unexpected list: %+v", list)
	}

	resp = h.do(t, http.MethodPost, "/api/admin/market/demo/install", "admin", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("install: %d %s", resp.Status, resp.Body)
	}
	var installed struct {
		Version string `json:"version"`
		Updated bool   `json:"updated"`
	}
	mustDecode(t, resp.Body, &installed)
	if installed.Version != "0.9.0" || installed.Updated {
		t.Fatalf("install result: %+v", installed)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "demo")); err != nil {
		t.Fatalf("binary not in plugins dir: %v", err)
	}

	// The protocol is live without a restart.
	resp = h.do(t, http.MethodGet, "/api/admin/protocols", "admin", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"demo"`) {
		t.Fatalf("protocols after install: %d %s", resp.Status, resp.Body)
	}

	// Installing again exercises the update path.
	resp = h.do(t, http.MethodPost, "/api/admin/market/demo/install", "admin", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("update: %d %s", resp.Status, resp.Body)
	}
	mustDecode(t, resp.Body, &installed)
	if !installed.Updated {
		t.Fatalf("second install must report an update: %+v", installed)
	}

	// A name owned by a built-in is rejected.
	resp = h.do(t, http.MethodPost, "/api/admin/market/ssh/install", "admin", nil)
	if resp.Status == http.StatusOK {
		t.Fatal("installing over a built-in name must fail")
	}
}

func mustDecode(t *testing.T, body []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
}
