package podman

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("podman manifest invalid: %v", err)
	}
}

func TestManifestDeclaresPodmanWorkspace(t *testing.T) {
	m := New().Manifest()
	if m.Layout != plugin.LayoutSidebarTree {
		t.Fatalf("layout = %q, want sidebar_tree", m.Layout)
	}
	if m.SupportsTransport(plugin.TransportDirect) {
		t.Fatal("podman must not declare direct transport")
	}
	if !m.SupportsTransport(plugin.TransportAgent) || m.Agent == nil {
		t.Fatal("podman must declare agent transport and profile")
	}
	if m.Agent.Proxy.Address != "/run/podman/podman.sock" {
		t.Fatalf("agent proxy address = %q, want podman socket", m.Agent.Proxy.Address)
	}
	if len(m.Tree) != 6 || len(m.Resources) != 6 {
		t.Fatalf("tree=%d resources=%d, want 6/6", len(m.Tree), len(m.Resources))
	}
	var hasPods bool
	for _, g := range m.Tree {
		if g.Key == "pods" {
			hasPods = true
		}
	}
	if !hasPods {
		t.Fatal("podman tree must include a pods group")
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Type == plugin.PanelHTTPClient {
				t.Fatalf("podman should not expose a raw API panel: resource=%s tab=%s", res.Kind, tab.Key)
			}
			if tab.Type == plugin.PanelTerminalGrid {
				t.Fatalf("podman should keep exec sessions as single terminal panels: resource=%s tab=%s", res.Kind, tab.Key)
			}
			if tab.Key == "inspect" {
				if tab.Type != plugin.PanelObjectDetail {
					t.Fatalf("podman inspect should render object details: resource=%s panel=%s", res.Kind, tab.Type)
				}
				if cfg, ok := tab.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
					t.Fatalf("podman inspect config for %s = %#v, want raw-toggle object detail", res.Kind, tab.Config)
				}
			}
		}
	}
	for _, route := range New().Routes() {
		if route.ID == "podman.api.execute" {
			t.Fatal("podman should not expose a raw API execute route")
		}
		if route.ID == "podman.container.open" {
			if route.Input == nil {
				t.Fatal("podman container open route should declare port input")
			}
			field := route.Input.Groups[0].Fields[0]
			if field.Type != plugin.FieldSelect || field.OptionsSource == nil || field.OptionsSource.RouteID != "podman.container.open.ports" {
				t.Fatalf("podman open port field should be sourced select: %+v", field)
			}
			if field.Required {
				t.Fatal("podman open port is a URL route param and must not make the GET body schema required")
			}
			if err := route.Input.ValidateValues(map[string]any{}, nil); err != nil {
				t.Fatalf("podman open route input should allow fallback port selection: %v", err)
			}
		}
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
	wantTabs := []string{"overview", "logs", "terminal", "env", "mounts", "inspect"}
	if len(containerRes.Detail.Tabs) != len(wantTabs) {
		t.Fatalf("container detail tabs = %d, want %d", len(containerRes.Detail.Tabs), len(wantTabs))
	}
	for i, want := range wantTabs {
		if containerRes.Detail.Tabs[i].Key != want {
			t.Fatalf("container tab %d = %q, want %q", i, containerRes.Detail.Tabs[i].Key, want)
		}
	}
	if terminal := containerRes.Detail.Tabs[2]; terminal.Type != plugin.PanelTerminal || terminal.VisibleWhen == nil {
		t.Fatalf("container exec panel = %s visible=%v, want running-only terminal", terminal.Type, terminal.VisibleWhen)
	}
	if mounts := containerRes.Detail.Tabs[4]; mounts.Type != plugin.PanelTable || mounts.Source.RouteID != "podman.container.mounts" {
		t.Fatalf("container mounts should render a table from mounts route, got panel=%s source=%+v", mounts.Type, mounts.Source)
	}
	for _, id := range []string{"podman.container.remove", "podman.pod.remove", "podman.image.remove", "podman.volume.remove", "podman.network.remove"} {
		action := findAction(m.Actions, id)
		if action == nil || action.OnSuccess == nil || action.OnSuccess.Navigate != plugin.NavigateList {
			t.Fatalf("%s should return to the resource list after success: %+v", id, action)
		}
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Type != plugin.PanelTable {
				continue
			}
			cfg, ok := tab.Config.(plugin.TableConfig)
			if !ok || cfg.EmptyText == "" {
				t.Fatalf("table %s/%s config = %#v, want empty text", res.Kind, tab.Key, tab.Config)
			}
		}
	}
}

func TestPodsAndContainersAgainstFakeDaemon(t *testing.T) {
	srv := fakePodmanDaemon(t)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host, port, _ := net.SplitHostPort(u.Host)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportAgent,
		Config:    map[string]any{"endpoint_type": "tcp", "host": host, "port": mustPort(t, port)},
		Net:       directNet{},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	rc := func(params map[string]string) *plugin.RequestContext {
		return plugin.NewRequestContext(context.Background(), plugin.User{ID: "u"}, sess, params, url.Values{}, nil)
	}

	pods, err := listPods(rc(nil))
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	podPage := pods.(plugin.Page[dockerengine.Row])
	if len(podPage.Items) != 1 || podPage.Items[0]["name"] != "web" || podPage.Items[0]["containers"] != 2 {
		t.Fatalf("pod row unexpected: %+v", podPage.Items)
	}

	ctrs, err := podContainers(rc(map[string]string{"id": "p1"}))
	if err != nil {
		t.Fatalf("pod containers: %v", err)
	}
	ctrPage := ctrs.(plugin.Page[dockerengine.Row])
	if len(ctrPage.Items) != 2 || ctrPage.Items[0]["name"] != "web-1" {
		t.Fatalf("pod containers unexpected: %+v", ctrPage.Items)
	}

	// Compat objects reuse the shared engine handlers over Podman's socket.
	list, err := dockerengine.ListContainers(rc(nil))
	if err != nil {
		t.Fatalf("list containers: %v", err)
	}
	if len(list.(plugin.Page[dockerengine.Row]).Items) != 1 {
		t.Fatalf("container list unexpected: %+v", list)
	}
}

func fakePodmanDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dockerAPIPath(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch p {
		case "/_ping":
			w.Header().Set("Api-Version", "1.41")
			_, _ = w.Write([]byte("OK"))
		case "/containers/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Id": "c1", "Names": []string{"/web-1"}, "Image": "nginx", "State": "running", "Status": "Up", "Created": float64(1710000000),
			}})
		case "/libpod/pods/json":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"Id":         "p1",
				"Name":       "web",
				"Status":     "Running",
				"Created":    "2024-01-01T00:00:00Z",
				"Containers": []map[string]any{{"Id": "c1"}, {"Id": "c2"}},
			}})
		case "/libpod/pods/p1/json":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":      "p1",
				"Name":    "web",
				"State":   "Running",
				"Created": "2024-01-01T00:00:00Z",
				"Containers": []map[string]any{
					{"Id": "c1", "Name": "web-1", "State": "running"},
					{"Id": "c2", "Name": "web-infra", "State": "running"},
				},
			})
		default:
			t.Logf("unexpected podman request %s %s", r.Method, p)
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(h)
}

type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func mustPort(t *testing.T, port string) int {
	t.Helper()
	n, err := net.LookupPort("tcp", port)
	if err != nil {
		t.Fatalf("parse port %q: %v", port, err)
	}
	return n
}

var versionPrefix = regexp.MustCompile(`^/v[0-9]+(\.[0-9]+){1,2}`)

func dockerAPIPath(path string) string {
	return versionPrefix.ReplaceAllString(path, "")
}

func findAction(actions []plugin.Action, id string) *plugin.Action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}
