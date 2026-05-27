package proxmox

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func TestNodeStorageTabIsNodeScoped(t *testing.T) {
	var storageTab *plugin.Tab
	for _, r := range New().Manifest().Resources {
		if r.Kind != "node" {
			continue
		}
		for i := range r.Detail.Tabs {
			if r.Detail.Tabs[i].Key == "storage" {
				storageTab = &r.Detail.Tabs[i]
			}
		}
	}
	if storageTab == nil || storageTab.Source == nil {
		t.Fatalf("node storage tab missing source")
	}
	if storageTab.Source.RouteID != "proxmox.node.storage" {
		t.Fatalf("node storage route = %q", storageTab.Source.RouteID)
	}
	if storageTab.Source.Params["node"] != "${resource.uid}" {
		t.Fatalf("node storage params = %+v", storageTab.Source.Params)
	}
}

func TestParseConnectOptions(t *testing.T) {
	cases := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
		check   func(connectOptions) bool
	}{
		{
			name: "token",
			cfg:  map[string]any{"host": "pve", "auth": "token", "token_id": "root@pam!ci", "token_secret": "s"},
			check: func(o connectOptions) bool {
				return o.Method == authToken && o.TokenID == "root@pam!ci" && o.Addr == "pve:8006"
			},
		},
		{
			name:  "password",
			cfg:   map[string]any{"host": "pve", "port": float64(443), "auth": "password", "username": "root@pam", "password": "p"},
			check: func(o connectOptions) bool { return o.Method == authPassword && o.Addr == "pve:443" },
		},
		{
			name: "credential",
			cfg: map[string]any{
				"host": "pve", "auth": "credential",
				service.CredentialIdentity: "root@pam!stored", service.CredentialSecret: "secret",
			},
			check: func(o connectOptions) bool { return o.Method == authToken && o.TokenID == "root@pam!stored" },
		},
		{name: "missing host", cfg: map[string]any{"auth": "token", "token_id": "x", "token_secret": "y"}, wantErr: true},
		{name: "token without secret", cfg: map[string]any{"host": "pve", "auth": "token", "token_id": "x"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseConnectOptions(plugin.ConnectConfig{Config: tc.cfg})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.check(opts) {
				t.Fatalf("unexpected options: %+v", opts)
			}
		})
	}
}

func TestTermFraming(t *testing.T) {
	if got := string(inputFrame([]byte("ls\n"))); got != "0:3:ls\n" {
		t.Fatalf("inputFrame = %q", got)
	}
	if got := string(resizeFrame(80, 24)); got != "1:24:80:" {
		t.Fatalf("resizeFrame = %q (want rows:cols)", got)
	}
}

func TestMetricFrame(t *testing.T) {
	guest := metricFrame(row{"cpu": 0.5, "mem": float64(512), "maxmem": float64(1024)})
	if guest["cpu"] != 50.0 || guest["mem"] != 50.0 {
		t.Fatalf("guest metric = %+v", guest)
	}
	node := metricFrame(row{"cpu": 0.1, "memory": map[string]any{"used": float64(1), "total": float64(4)}})
	if node["cpu"] != 10.0 || node["mem"] != 25.0 {
		t.Fatalf("node metric = %+v", node)
	}
}

func TestRoutesAgainstFakeProxmox(t *testing.T) {
	srv := fakeProxmox(t)
	defer srv.Close()

	host, port := splitHostPort(t, srv.URL)
	sess := dialSession(t, host, port)

	t.Run("list qemu", func(t *testing.T) {
		page := callList(t, sess, listGuests("qemu"), nil)
		if len(page.Items) != 1 || page.Items[0]["name"] != "web" {
			t.Fatalf("qemu rows = %+v", page.Items)
		}
		ref := page.Items[0]["ref"].(plugin.ResourceRef)
		if ref.Namespace != "pve" || ref.UID != "100" {
			t.Fatalf("qemu ref = %+v", ref)
		}
	})

	t.Run("list lxc", func(t *testing.T) {
		page := callList(t, sess, listGuests("lxc"), nil)
		if len(page.Items) != 1 || page.Items[0]["name"] != "ct1" {
			t.Fatalf("lxc rows = %+v", page.Items)
		}
	})

	t.Run("snapshot ref packs node/vmid/name", func(t *testing.T) {
		page := callList(t, sess, listSnapshots("qemu"), map[string]string{"node": "pve", "vmid": "100"})
		if len(page.Items) != 1 {
			t.Fatalf("snapshot rows = %+v", page.Items)
		}
		ref := page.Items[0]["ref"].(plugin.ResourceRef)
		if ref.Namespace != "pve" || ref.Name != "100" || ref.UID != "pre-upgrade" {
			t.Fatalf("snapshot ref = %+v", ref)
		}
	})

	t.Run("tree node children", func(t *testing.T) {
		result, err := treeNodeChildren(newRC(sess, map[string]string{"node": "pve"}))
		if err != nil {
			t.Fatalf("tree: %v", err)
		}
		page := result.(plugin.Page[plugin.TreeNode])
		if len(page.Items) < 2 {
			t.Fatalf("expected guests + storage, got %+v", page.Items)
		}
	})

	t.Run("list node storage", func(t *testing.T) {
		page := callList(t, sess, listNodeStorage, map[string]string{"node": "pve"})
		if len(page.Items) != 1 || page.Items[0]["name"] != "local" {
			t.Fatalf("storage rows = %+v", page.Items)
		}
		ref := page.Items[0]["ref"].(plugin.ResourceRef)
		if ref.Kind != "storage" || ref.Namespace != "pve" || ref.UID != "local" {
			t.Fatalf("storage ref = %+v", ref)
		}
	})
}

// --- helpers --------------------------------------------------------------

func fakeProxmox(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api2/json/version", jsonHandler(`{"data":{"version":"8.1.0","release":"8"}}`))
	mux.HandleFunc("/api2/json/cluster/resources", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") == "storage" {
			_, _ = w.Write([]byte(`{"data":[{"type":"storage","storage":"local","node":"pve","plugintype":"dir","content":"backup,iso","disk":10,"maxdisk":100}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[
			{"type":"qemu","vmid":100,"name":"web","node":"pve","status":"running","cpu":0.25,"mem":1073741824,"maxmem":2147483648,"uptime":3600},
			{"type":"lxc","vmid":200,"name":"ct1","node":"pve","status":"stopped","cpu":0,"mem":0,"maxmem":536870912}
		]}`))
	})
	mux.HandleFunc("/api2/json/nodes/pve/storage", jsonHandler(`{"data":[{"storage":"local","type":"dir","content":"backup,iso","used":10,"total":100,"active":1}]}`))
	mux.HandleFunc("/api2/json/nodes/pve/qemu/100/snapshot", jsonHandler(`{"data":[{"name":"pre-upgrade","description":"before update","snaptime":1700000000,"parent":""}]}`))
	srv := httptest.NewTLSServer(mux)
	return srv
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func dialSession(t *testing.T, host, port string) *Session {
	t.Helper()
	cfg := plugin.ConnectConfig{
		Net: directNet{},
		Config: map[string]any{
			"host": host, "port": atoi(port), "verify_tls": false,
			"auth": "token", "token_id": "root@pam!test", "token_secret": "secret",
		},
	}
	s, err := connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return s.(*Session)
}

func callList(t *testing.T, sess *Session, h plugin.Handler, params map[string]string) plugin.Page[row] {
	t.Helper()
	result, err := h(newRC(sess, params))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	page, ok := result.(plugin.Page[row])
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	return page
}

func newRC(sess *Session, params map[string]string) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), models.User{}, sess, params, url.Values{}, nil)
}

// directNet dials the literal address — the fake server runs on the configured
// host:port, so the same transport the gateway wires is exercised end to end.
type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func splitHostPort(t *testing.T, raw string) (host, port string) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	host, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host: %v", err)
	}
	return host, port
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
