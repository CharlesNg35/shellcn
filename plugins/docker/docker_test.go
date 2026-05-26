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
	if !contains(containerRes.ListActionIDs, "docker.container.create") {
		t.Fatalf("container list actions = %#v, want create action", containerRes.ListActionIDs)
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
	if containerRes.Detail.Tabs[0].Panel != plugin.PanelDocument || containerRes.Detail.Tabs[0].Source.RouteID != "docker.container.overview" {
		t.Fatalf("container overview should render selected container details, got panel=%s source=%+v", containerRes.Detail.Tabs[0].Panel, containerRes.Detail.Tabs[0].Source)
	}
	var composeRes *plugin.ResourceType
	for i := range m.Resources {
		if m.Resources[i].Kind == "compose" {
			composeRes = &m.Resources[i]
			break
		}
	}
	if composeRes == nil {
		t.Fatal("missing compose resource")
	}
	wantComposeTabs := []string{"overview", "containers", "services", "api"}
	if len(composeRes.Detail.Tabs) != len(wantComposeTabs) {
		t.Fatalf("compose detail tabs = %d, want %d", len(composeRes.Detail.Tabs), len(wantComposeTabs))
	}
	for i, want := range wantComposeTabs {
		if composeRes.Detail.Tabs[i].Key != want {
			t.Fatalf("compose tab %d = %q, want %q", i, composeRes.Detail.Tabs[i].Key, want)
		}
	}
	var createAction *plugin.Action
	for i := range m.Actions {
		if m.Actions[i].ID == "docker.container.create" {
			createAction = &m.Actions[i]
			break
		}
	}
	if createAction == nil || createAction.RouteID != "docker.container.create" {
		t.Fatalf("missing create container action: %+v", createAction)
	}
	var createRoute *plugin.Route
	routes := New().Routes()
	for i := range routes {
		if routes[i].ID == "docker.container.create" {
			createRoute = &routes[i]
			break
		}
	}
	if createRoute == nil || createRoute.Input == nil || createRoute.Risk != plugin.RiskWrite {
		t.Fatalf("create container route mismatch: %+v", createRoute)
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

	overview, err := containerOverview(inspectRC)
	if err != nil {
		t.Fatalf("container overview: %v", err)
	}
	if fmt.Sprint(overview.(row)["name"]) != "web" || fmt.Sprint(overview.(row)["state"]) != "running" {
		t.Fatalf("container overview unexpected: %+v", overview)
	}

	composeRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, map[string]string{"project": "demo"}, url.Values{}, nil)
	services, err := composeServices(composeRC)
	if err != nil {
		t.Fatalf("compose services: %v", err)
	}
	servicePage := services.(plugin.Page[row])
	if len(servicePage.Items) != 1 || servicePage.Items[0]["name"] != "web" || servicePage.Items[0]["running"] != 1 {
		t.Fatalf("compose services unexpected: %+v", servicePage.Items)
	}

	if _, err := startContainer(inspectRC); err != nil {
		t.Fatalf("start container: %v", err)
	}
	if !calls["POST /containers/abc123/start"] {
		t.Fatalf("start endpoint not called: %+v", calls)
	}

	createBody := `{"name":"api","image":"nginx:latest","pull":false,"start":true,"command":"nginx -g 'daemon off;'","env":"APP_ENV=test","ports":"8080:80/tcp","binds":"/srv/app:/app:ro","network":"bridge","restart":"unless-stopped"}`
	createRC := plugin.NewRequestContext(context.Background(), models.User{ID: "u"}, sess, nil, url.Values{}, []byte(createBody))
	created, err := createContainer(createRC)
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	createResult := created.(createContainerResult)
	if !createResult.OK || createResult.ID != "def456789abc" || !createResult.Started {
		t.Fatalf("create result unexpected: %+v", createResult)
	}
	if !calls["POST /containers/create"] || !calls["POST /containers/def456789abcdef/start"] {
		t.Fatalf("create/start endpoints not called: %+v", calls)
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

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
				"Labels":  map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": "web"},
			}})
		case p == "/containers/abc123/json":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Id":    "abc123",
				"Name":  "/web",
				"Image": "sha256:img",
				"Config": map[string]any{
					"Tty":    false,
					"Env":    []string{"APP_ENV=prod"},
					"Labels": map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": "web"},
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
		case r.Method == http.MethodPost && p == "/containers/create":
			if got := r.URL.Query().Get("name"); got != "api" {
				t.Errorf("container create name query = %q, want api", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode create body: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if body["Image"] != "nginx:latest" {
				t.Errorf("create image = %#v", body["Image"])
			}
			env, _ := body["Env"].([]any)
			if len(env) != 1 || env[0] != "APP_ENV=test" {
				t.Errorf("create env = %#v", body["Env"])
			}
			cmd, _ := body["Cmd"].([]any)
			if len(cmd) != 3 || cmd[2] != "daemon off;" {
				t.Errorf("create command = %#v", body["Cmd"])
			}
			host, _ := body["HostConfig"].(map[string]any)
			if host["NetworkMode"] != "bridge" {
				t.Errorf("network mode = %#v", host["NetworkMode"])
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": "def456789abcdef", "Warnings": []string{"created"}})
		case r.Method == http.MethodPost && p == "/containers/def456789abcdef/start":
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
