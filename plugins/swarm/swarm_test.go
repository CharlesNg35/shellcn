package swarm

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"testing"

	"github.com/moby/moby/api/types/swarm"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("swarm manifest invalid: %v", err)
	}
}

func TestManifestDeclaresSwarmWorkspace(t *testing.T) {
	m := New().Manifest()
	if m.Layout != plugin.LayoutSidebarTree {
		t.Fatalf("layout = %q, want sidebar_tree", m.Layout)
	}
	if !m.SupportsTransport(plugin.TransportAgent) || m.Agent == nil {
		t.Fatal("swarm must declare agent transport and profile")
	}
	if len(m.Tree) != 5 || len(m.Resources) != 5 {
		t.Fatalf("tree=%d resources=%d, want 5/5", len(m.Tree), len(m.Resources))
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Type == plugin.PanelHTTPClient {
				t.Fatalf("swarm should not expose a raw API panel: resource=%s tab=%s", res.Kind, tab.Key)
			}
			if tab.Key == "inspect" {
				if tab.Type != plugin.PanelObjectDetail {
					t.Fatalf("swarm inspect should render object details: resource=%s panel=%s", res.Kind, tab.Type)
				}
				if cfg, ok := tab.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
					t.Fatalf("swarm inspect config for %s = %#v, want raw-toggle object detail", res.Kind, tab.Config)
				}
			}
		}
	}
	for _, route := range New().Routes() {
		if route.ID == "swarm.api.execute" {
			t.Fatal("swarm should not expose a raw API execute route")
		}
		if route.ID == "swarm.service.open" {
			if route.Input == nil {
				t.Fatal("swarm service open route should declare port input")
			}
			field := route.Input.Groups[0].Fields[0]
			if field.Type != plugin.FieldSelect || field.OptionsSource == nil || field.OptionsSource.RouteID != "swarm.service.open.ports" {
				t.Fatalf("swarm open port field should be sourced select: %+v", field)
			}
			if field.Required {
				t.Fatal("swarm open port is a URL route param and must not make the GET body schema required")
			}
			if err := route.Input.ValidateValues(map[string]any{}, nil); err != nil {
				t.Fatalf("swarm open route input should allow fallback port selection: %v", err)
			}
		}
	}
	for _, id := range []string{"swarm.service.update", "swarm.service.rollback", "swarm.service.remove", "swarm.node.update", "swarm.stack.deploy", "swarm.stack.remove"} {
		action := findAction(m.Actions, id)
		if action == nil || !action.Confirm || action.ConfirmText == "" {
			t.Fatalf("%s should declare a confirmation: %+v", id, action)
		}
	}
	for _, id := range []string{"swarm.service.remove", "swarm.stack.remove"} {
		action := findAction(m.Actions, id)
		if action == nil || action.OnSuccess == nil || action.OnSuccess.Navigate != plugin.NavigateList {
			t.Fatalf("%s should return to the resource list after success: %+v", id, action)
		}
	}
	var stackRes *plugin.ResourceType
	for i := range m.Resources {
		if m.Resources[i].Kind == "stack" {
			stackRes = &m.Resources[i]
			break
		}
	}
	if stackRes == nil || !contains(stackRes.Actions.Row, "swarm.stack.remove") || !contains(stackRes.Actions.Detail, "swarm.stack.remove") {
		t.Fatalf("stack resource should expose remove action: %+v", stackRes)
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

func TestRoutesAgainstFakeSwarmDaemon(t *testing.T) {
	srv := fakeSwarmDaemon(t)
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
	rc := func(params map[string]string) *plugin.RequestContext {
		return plugin.NewRequestContext(context.Background(), plugin.User{ID: "u"}, sess, params, url.Values{}, nil)
	}

	services, err := listServices(rc(nil))
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	svc := services.(plugin.Page[dockerengine.Row])
	if len(svc.Items) != 1 || svc.Items[0]["name"] != "web" || svc.Items[0]["replicas"] != "2/3" || svc.Items[0]["mode"] != "replicated" || svc.Items[0]["stack"] != "demo" {
		t.Fatalf("service row unexpected: %+v", svc.Items)
	}

	nodes, err := listNodes(rc(nil))
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	node := nodes.(plugin.Page[dockerengine.Row])
	if len(node.Items) != 1 || node.Items[0]["name"] != "mgr1" || node.Items[0]["role"] != "manager" || node.Items[0]["leader"] != true {
		t.Fatalf("node row unexpected: %+v", node.Items)
	}

	tasks, err := listTasks(rc(nil))
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	task := tasks.(plugin.Page[dockerengine.Row])
	if len(task.Items) != 1 || task.Items[0]["name"] != "web.1" || task.Items[0]["state"] != "running" {
		t.Fatalf("task row unexpected: %+v", task.Items)
	}

	stacks, err := listStacks(rc(nil))
	if err != nil {
		t.Fatalf("list stacks: %v", err)
	}
	stack := stacks.(plugin.Page[dockerengine.Row])
	if len(stack.Items) != 1 || stack.Items[0]["name"] != "demo" || stack.Items[0]["services"] != 1 {
		t.Fatalf("stack row unexpected: %+v", stack.Items)
	}
}

func u64(v uint64) *uint64 { return &v }

func TestServicePortOptions(t *testing.T) {
	got := servicePortOptions([]swarm.PortConfig{
		{Protocol: "tcp", TargetPort: 80, PublishedPort: 8080, PublishMode: swarm.PortConfigPublishModeHost},
		{Name: "https", Protocol: "tcp", TargetPort: 8443, PublishedPort: 9443, PublishMode: swarm.PortConfigPublishModeIngress},
		{Protocol: "udp", TargetPort: 53, PublishedPort: 53, PublishMode: swarm.PortConfigPublishModeIngress},
	})
	if len(got) != 1 {
		t.Fatalf("servicePortOptions = %#v", got)
	}
	if got[0].Value != "https:9443" || got[0].Label != "https - HTTPS 9443->8443/tcp" {
		t.Fatalf("servicePortOptions[0] = %+v", got[0])
	}
}

func TestParseAvailability(t *testing.T) {
	for _, in := range []string{"active", "PAUSE", " drain "} {
		if _, err := parseAvailability(in); err != nil {
			t.Fatalf("parseAvailability(%q): %v", in, err)
		}
	}
	if _, err := parseAvailability("offline"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("parseAvailability(offline) err = %v, want ErrInvalidInput", err)
	}
}

func TestParseRole(t *testing.T) {
	if r, err := parseRole("Manager"); err != nil || r != swarm.NodeRoleManager {
		t.Fatalf("parseRole(Manager) = %q, %v", r, err)
	}
	if _, err := parseRole("leader"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("parseRole(leader) err = %v, want ErrInvalidInput", err)
	}
}

func TestParseEnv(t *testing.T) {
	got, err := parseEnv("FOO=bar\n\n  BAZ=qux  \n")
	if err != nil {
		t.Fatalf("parseEnv: %v", err)
	}
	want := []string{"FOO=bar", "BAZ=qux"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseEnv = %#v, want %#v", got, want)
	}
	if _, err := parseEnv("noequals"); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("parseEnv(noequals) err = %v, want ErrInvalidInput", err)
	}
}

func TestApplyServiceUpdate(t *testing.T) {
	env := "A=1\nB=2"
	spec := swarm.ServiceSpec{
		TaskTemplate: swarm.TaskSpec{ContainerSpec: &swarm.ContainerSpec{Image: "nginx:1.0"}},
		Mode:         swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: u64(1)}},
	}
	if err := applyServiceUpdate(&spec, serviceUpdateRequest{Image: "nginx:2.0", Env: &env, Replicas: u64(5)}); err != nil {
		t.Fatalf("applyServiceUpdate: %v", err)
	}
	if spec.TaskTemplate.ContainerSpec.Image != "nginx:2.0" {
		t.Fatalf("image = %q", spec.TaskTemplate.ContainerSpec.Image)
	}
	if got := *spec.Mode.Replicated.Replicas; got != 5 {
		t.Fatalf("replicas = %d, want 5", got)
	}
	if !reflect.DeepEqual(spec.TaskTemplate.ContainerSpec.Env, []string{"A=1", "B=2"}) {
		t.Fatalf("env = %#v", spec.TaskTemplate.ContainerSpec.Env)
	}
}

func TestApplyServiceUpdateNoChange(t *testing.T) {
	spec := swarm.ServiceSpec{TaskTemplate: swarm.TaskSpec{ContainerSpec: &swarm.ContainerSpec{Image: "nginx:1.0"}}}
	if err := applyServiceUpdate(&spec, serviceUpdateRequest{}); err != nil {
		t.Fatalf("applyServiceUpdate: %v", err)
	}
	if spec.TaskTemplate.ContainerSpec.Image != "nginx:1.0" {
		t.Fatalf("image changed unexpectedly: %q", spec.TaskTemplate.ContainerSpec.Image)
	}
}

func TestApplyServiceUpdateReplicasOnGlobalFails(t *testing.T) {
	spec := swarm.ServiceSpec{Mode: swarm.ServiceMode{Global: &swarm.GlobalService{}}}
	if err := applyServiceUpdate(&spec, serviceUpdateRequest{Replicas: u64(3)}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestSwarmActionSchemasGuideBoundedInput(t *testing.T) {
	update := serviceUpdateSchema()
	image := requireSchemaField(t, update, "image")
	if image.Type != plugin.FieldAutocomplete {
		t.Fatalf("service image field type = %q, want autocomplete", image.Type)
	}
	if err := update.ValidateValues(map[string]any{"replicas": float64(10001)}, nil); err == nil {
		t.Fatal("service update schema accepted too many replicas")
	}

	node := nodeUpdateSchema()
	availability := requireSchemaField(t, node, "availability")
	if availability.Type != plugin.FieldSelect {
		t.Fatalf("availability field type = %q, want select", availability.Type)
	}
	if err := node.ValidateValues(map[string]any{"availability": "offline"}, nil); err == nil {
		t.Fatal("node update schema accepted an invalid availability")
	}

	deploy := stackDeploySchema()
	if err := deploy.ValidateValues(map[string]any{"name": "bad name", "spec": []any{}}, nil); err == nil {
		t.Fatal("stack deploy schema accepted a whitespace-bearing name")
	}
}

func TestStampStackNamespace(t *testing.T) {
	spec := swarm.ServiceSpec{}
	stampStackNamespace(&spec, "demo")
	if spec.Labels[stackNamespaceLabel] != "demo" {
		t.Fatalf("namespace label = %q", spec.Labels[stackNamespaceLabel])
	}
}

func findAction(actions []plugin.Action, id string) *plugin.Action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func requireSchemaField(t *testing.T, schema *plugin.Schema, key string) *plugin.Field {
	t.Helper()
	for i := range schema.Groups {
		for j := range schema.Groups[i].Fields {
			field := &schema.Groups[i].Fields[j]
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("schema missing %q field", key)
	return nil
}

func fakeSwarmDaemon(t *testing.T) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dockerAPIPath(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch p {
		case "/_ping":
			w.Header().Set("Api-Version", "1.54")
			_, _ = w.Write([]byte("OK"))
		case "/services":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"ID":        "svc1",
				"CreatedAt": "2024-01-01T00:00:00Z",
				"Spec": map[string]any{
					"Name":         "web",
					"Labels":       map[string]string{"com.docker.stack.namespace": "demo"},
					"Mode":         map[string]any{"Replicated": map[string]any{"Replicas": 3}},
					"TaskTemplate": map[string]any{"ContainerSpec": map[string]any{"Image": "nginx:latest@sha256:abc"}},
				},
				"Endpoint":      map[string]any{"Ports": []map[string]any{{"Protocol": "tcp", "TargetPort": 80, "PublishedPort": 8080}}},
				"ServiceStatus": map[string]any{"RunningTasks": 2, "DesiredTasks": 3},
			}})
		case "/nodes":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"ID":        "node1",
				"CreatedAt": "2024-01-01T00:00:00Z",
				"Spec":      map[string]any{"Role": "manager", "Availability": "active"},
				"Description": map[string]any{
					"Hostname":  "mgr1",
					"Platform":  map[string]any{"OS": "linux", "Architecture": "x86_64"},
					"Engine":    map[string]any{"EngineVersion": "28.5.2"},
					"Resources": map[string]any{"NanoCPUs": 4000000000, "MemoryBytes": 8000000000},
				},
				"Status":        map[string]any{"State": "ready", "Addr": "10.0.0.1"},
				"ManagerStatus": map[string]any{"Leader": true, "Reachability": "reachable", "Addr": "10.0.0.1:2377"},
			}})
		case "/tasks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"ID":           "task1",
				"CreatedAt":    "2024-01-01T00:00:00Z",
				"ServiceID":    "svc1",
				"NodeID":       "node1",
				"Slot":         1,
				"DesiredState": "running",
				"Status":       map[string]any{"State": "running"},
				"Spec":         map[string]any{"ContainerSpec": map[string]any{"Image": "nginx:latest"}},
			}})
		default:
			t.Logf("unexpected swarm request %s %s", r.Method, p)
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

var versionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func dockerAPIPath(path string) string {
	return versionPrefix.ReplaceAllString(path, "")
}
