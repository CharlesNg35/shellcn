package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("docker manifest invalid: %v", err)
	}
}

func TestManifestDeclaresDockerWorkspace(t *testing.T) {
	m := New().Manifest()
	if m.Layout != plugin.LayoutSidebarTree {
		t.Fatalf("layout = %q, want sidebar_tree", m.Layout)
	}
	if !m.SupportsTransport(plugin.TransportAgent) || m.Agent == nil {
		t.Fatal("docker must declare agent transport and profile")
	}
	if m.Agent.Proxy.Mode != plugin.AgentUnix || m.Agent.Proxy.Address != "/var/run/docker.sock" || m.Agent.Proxy.Risk != plugin.RiskPrivileged {
		t.Fatalf("agent proxy mismatch: %+v", m.Agent.Proxy)
	}
	if len(m.Tree) != 5 {
		t.Fatalf("tree groups = %d, want 5", len(m.Tree))
	}
	if len(m.Resources) != 5 {
		t.Fatalf("resources = %d, want 5", len(m.Resources))
	}
	var containerRes *plugin.ResourceType
	for i := range m.Resources {
		if m.Resources[i].Kind == "container" {
			containerRes = &m.Resources[i]
			break
		}
	}
	if containerRes == nil {
		t.Fatal("missing container resource")
	}
	wantTabs := []string{"overview", "terminal", "logs", "inspect", "env", "api"}
	if len(containerRes.Detail.Tabs) != len(wantTabs) {
		t.Fatalf("container detail tabs = %d, want %d", len(containerRes.Detail.Tabs), len(wantTabs))
	}
	for i, want := range wantTabs {
		if containerRes.Detail.Tabs[i].Key != want {
			t.Fatalf("tab %d = %q, want %q", i, containerRes.Detail.Tabs[i].Key, want)
		}
	}
}

func TestConfigSchemaHidesEndpointForAgentTransport(t *testing.T) {
	schema := configSchema()
	direct := map[string]any{plugin.SchemaContextTransport: string(plugin.TransportDirect)}
	if err := schema.ValidateValuesWithContext(map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"}, nil, direct); err != nil {
		t.Fatalf("direct unix config rejected: %v", err)
	}
	agent := map[string]any{plugin.SchemaContextTransport: string(plugin.TransportAgent)}
	if err := schema.ValidateValuesWithContext(map[string]any{}, nil, agent); err != nil {
		t.Fatalf("agent config should not require endpoint fields: %v", err)
	}
	if visible := schema.VisibleValues(map[string]any{"endpoint_type": "unix", "socket_path": "/var/run/docker.sock"}, agent); len(visible) != 0 {
		t.Fatalf("agent config should not persist direct endpoint fields: %#v", visible)
	}
}

func TestRoutesAgainstFakeDockerDaemon(t *testing.T) {
	srv, calls := fakeDockerDaemon(t)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Config: map[string]any{"endpoint_type": "tcp", "host": host, "port": mustPort(t, port)},
		Net:    directNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, nil)
	got, err := listContainers(rc)
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	page := got.(plugin.Page[row])
	if len(page.Items) != 1 || page.Items[0]["name"] != "web" {
		t.Fatalf("container page unexpected: %+v", page.Items)
	}

	inspectRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, map[string]string{"id": "abc123"}, url.Values{}, nil)
	doc, err := inspectContainer(inspectRC)
	if err != nil {
		t.Fatalf("inspect container: %v", err)
	}
	asMap := doc.(map[string]any)
	if asMap["Name"] != "/web" {
		t.Fatalf("inspect name = %#v", asMap["Name"])
	}

	if _, err := startContainer(inspectRC); err != nil {
		t.Fatalf("start container: %v", err)
	}
	if !calls["POST /containers/abc123/start"] {
		t.Fatalf("start endpoint not called: %+v", calls)
	}

	body := `{"method":"GET","url":"/version","headers":[]}`
	apiRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, []byte(body))
	raw, err := executeAPI(apiRC)
	if err != nil {
		t.Fatalf("execute api: %v", err)
	}
	resp := raw.(apiResponse)
	if resp.Status != http.StatusOK {
		t.Fatalf("raw api status = %d", resp.Status)
	}
}

type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func mustPort(t *testing.T, port string) int {
	t.Helper()
	var n int
	if _, err := fmt.Sscanf(port, "%d", &n); err != nil {
		t.Fatalf("parse port %q: %v", port, err)
	}
	return n
}

func fakeDockerDaemon(t *testing.T) (*httptest.Server, map[string]bool) {
	t.Helper()
	calls := map[string]bool{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dockerAPIPath(r.URL.Path)
		calls[r.Method+" "+p] = true
		w.Header().Set("Content-Type", "application/json")
		switch {
		case p == "/_ping":
			w.Header().Set("Api-Version", "1.54")
			_, _ = w.Write([]byte("OK"))
		case p == "/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": "28.5.2"})
		case p == "/containers/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Id":      "abc123",
				"Names":   []string{"/web"},
				"Image":   "nginx:latest",
				"ImageID": "sha256:img",
				"Command": "nginx",
				"Created": float64(1710000000),
				"State":   "running",
				"Status":  "Up 2 minutes",
				"Labels":  map[string]string{"com.docker.compose.project": "demo"},
			}})
		case p == "/containers/abc123/json":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":    "abc123",
				"Name":  "/web",
				"Image": "sha256:img",
				"Config": map[string]any{
					"Tty": false,
					"Env": []string{"APP_ENV=prod"},
				},
				"State": map[string]any{"Status": "running", "Running": true},
			})
		case p == "/images/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": "sha256:img", "RepoTags": []string{"nginx:latest"}, "Size": 1234, "Created": 1710000000, "Containers": 1}})
		case p == "/volumes":
			_ = json.NewEncoder(w).Encode(map[string]any{"Volumes": []map[string]any{{"Name": "data", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/data", "Scope": "local"}}})
		case p == "/networks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"Id": "net1", "Name": "bridge", "Driver": "bridge", "Scope": "local"}})
		case r.Method == http.MethodPost && p == "/containers/abc123/start":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Logf("unexpected docker request %s %s", r.Method, p)
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(h), calls
}

var versionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func dockerAPIPath(path string) string {
	return versionPrefix.ReplaceAllString(path, "")
}
