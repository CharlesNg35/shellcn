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
	wantTabs := []string{"overview", "logs", "terminal", "env", "mounts", "inspect"}
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
	if logs := containerRes.Detail.Tabs[1]; logs.Type != plugin.PanelLogStream {
		t.Fatalf("container logs panel = %s, want %s", logs.Type, plugin.PanelLogStream)
	}
	if terminal := containerRes.Detail.Tabs[2]; terminal.Type != plugin.PanelTerminal || terminal.Label != "Exec" {
		t.Fatalf("container exec panel = %s/%s, want Exec terminal", terminal.Type, terminal.Label)
	} else if terminal.VisibleWhen == nil {
		t.Fatalf("container exec panel should only be visible for running containers")
	}
	if env := containerRes.Detail.Tabs[3]; env.Type != plugin.PanelTable {
		t.Fatalf("container env should render a table, got %s", env.Type)
	} else if cfg, ok := env.Config.(plugin.TableConfig); !ok || cfg.EmptyText == "" {
		t.Fatalf("container env table config = %#v, want empty text", env.Config)
	}
	if mounts := containerRes.Detail.Tabs[4]; mounts.Type != plugin.PanelTable || mounts.Source.RouteID != "docker.container.mounts" {
		t.Fatalf("container mounts should render a table from mounts route, got panel=%s source=%+v", mounts.Type, mounts.Source)
	} else if cfg, ok := mounts.Config.(plugin.TableConfig); !ok || cfg.EmptyText == "" {
		t.Fatalf("container mounts table config = %#v, want empty text", mounts.Config)
	}
	if inspect := containerRes.Detail.Tabs[5]; inspect.Type != plugin.PanelObjectDetail || inspect.Source.RouteID != "docker.container.inspect" {
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
	for _, tab := range composeRes.Detail.Tabs[1:] {
		cfg, ok := tab.Config.(plugin.TableConfig)
		if !ok || cfg.EmptyText == "" {
			t.Fatalf("compose table %s config = %#v, want empty text", tab.Key, tab.Config)
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
	for _, id := range []string{"docker.container.remove", "docker.image.remove", "docker.volume.remove", "docker.network.remove", "docker.compose.down"} {
		action := findAction(m.Actions, id)
		if action == nil || action.OnSuccess == nil || action.OnSuccess.Navigate != plugin.NavigateList {
			t.Fatalf("%s should return to the resource list after success: %+v", id, action)
		}
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

func findAction(actions []plugin.Action, id string) *plugin.Action {
	for i := range actions {
		if actions[i].ID == id {
			return &actions[i]
		}
	}
	return nil
}

type directNet struct{}

func (directNet) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func (directNet) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }
