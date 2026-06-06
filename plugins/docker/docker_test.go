package docker

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
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
	if len(m.Tree) != 6 {
		t.Fatalf("tree groups = %d, want 6", len(m.Tree))
	}
	if len(m.Resources) != 6 {
		t.Fatalf("resources = %d, want 6", len(m.Resources))
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Type == plugin.PanelTerminalGrid {
				t.Fatalf("docker should keep exec sessions as single terminal panels: resource=%s tab=%s", res.Kind, tab.Key)
			}
			if tab.Key == "inspect" {
				if tab.Type != plugin.PanelObjectDetail {
					t.Fatalf("docker inspect should render object details: resource=%s panel=%s", res.Kind, tab.Type)
				}
				if cfg, ok := tab.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
					t.Fatalf("docker inspect config for %s = %#v, want raw-toggle object detail", res.Kind, tab.Config)
				}
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
	if !contains(containerRes.Actions.Toolbar, "docker.container.create") {
		t.Fatalf("container list actions = %#v, want create action", containerRes.Actions.Toolbar)
	}
	wantTabs := []string{"overview", "terminal", "logs", "inspect", "env"}
	if len(containerRes.Detail.Tabs) != len(wantTabs) {
		t.Fatalf("container detail tabs = %d, want %d", len(containerRes.Detail.Tabs), len(wantTabs))
	}
	for i, want := range wantTabs {
		if containerRes.Detail.Tabs[i].Key != want {
			t.Fatalf("tab %d = %q, want %q", i, containerRes.Detail.Tabs[i].Key, want)
		}
	}
	if containerRes.Detail.Tabs[0].Type != plugin.PanelObjectDetail || containerRes.Detail.Tabs[0].Source.RouteID != "docker.container.overview" {
		t.Fatalf("container overview should render selected container details, got panel=%s source=%+v", containerRes.Detail.Tabs[0].Type, containerRes.Detail.Tabs[0].Source)
	}
	if containerRes.Detail.Tabs[1].Type != plugin.PanelTerminal {
		t.Fatalf("container terminal panel = %s, want %s", containerRes.Detail.Tabs[1].Type, plugin.PanelTerminal)
	}
	if inspect := containerRes.Detail.Tabs[3]; inspect.Type != plugin.PanelObjectDetail || inspect.Source.RouteID != "docker.container.inspect" {
		t.Fatalf("container inspect should render object details, got panel=%s source=%+v", inspect.Type, inspect.Source)
	} else if cfg, ok := inspect.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
		t.Fatalf("container inspect config = %#v, want raw-toggle object detail", inspect.Config)
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
	wantComposeTabs := []string{"overview", "containers", "services"}
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
	var openRoute *plugin.Route
	for i := range routes {
		if routes[i].ID == "docker.container.open" {
			openRoute = &routes[i]
			break
		}
	}
	if openRoute == nil || openRoute.Input == nil {
		t.Fatalf("open container route should declare port input: %+v", openRoute)
	}
	openPort := openRoute.Input.Groups[0].Fields[0]
	if openPort.Type != plugin.FieldSelect || openPort.OptionsSource == nil || openPort.OptionsSource.RouteID != "docker.container.open.ports" {
		t.Fatalf("open port field should be sourced select: %+v", openPort)
	}
	if openPort.Required {
		t.Fatal("open port is a URL route param and must not make the GET body schema required")
	}
	if err := openRoute.Input.ValidateValues(map[string]any{}, nil); err != nil {
		t.Fatalf("open route input should allow fallback port selection: %v", err)
	}
	if !contains(m.HeaderActions, "docker.engine.shell") {
		t.Fatalf("header actions = %#v, want docker shell", m.HeaderActions)
	}
	var shellAction *plugin.Action
	for i := range m.Actions {
		if m.Actions[i].ID == "docker.engine.shell" {
			shellAction = &m.Actions[i]
			break
		}
	}
	if shellAction == nil || shellAction.Open != plugin.OpenDock || shellAction.Panel != plugin.PanelTerminal || !shellAction.Confirm {
		t.Fatalf("docker shell action mismatch: %+v", shellAction)
	}
	var shellStream *plugin.Stream
	for i := range m.Streams {
		if m.Streams[i].ID == "docker.engine.shell" {
			shellStream = &m.Streams[i]
			break
		}
	}
	if shellStream == nil || shellStream.Kind != plugin.StreamTerminal || shellStream.RouteID != "docker.engine.shell" {
		t.Fatalf("docker shell stream mismatch: %+v", shellStream)
	}
	var shellRoute *plugin.Route
	for i := range routes {
		if routes[i].ID == "docker.engine.shell" {
			shellRoute = &routes[i]
			break
		}
	}
	if shellRoute == nil || shellRoute.Permission != "docker.engine.shell" || shellRoute.Risk != plugin.RiskPrivileged || shellRoute.Method != plugin.MethodWS {
		t.Fatalf("docker shell route mismatch: %+v", shellRoute)
	}
	if shellRoute.Permission == "docker.containers.exec" {
		t.Fatal("docker engine shell must not reuse the container exec permission")
	}
	if len(m.Recording) != 1 || !contains(m.Recording[0].StreamIDs, "docker.engine.shell") {
		t.Fatalf("recording streams = %#v, want docker.engine.shell", m.Recording)
	}
	for i := range routes {
		if routes[i].ID == "docker.api.execute" {
			t.Fatal("docker should not expose a raw API execute route")
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
