// Package docker implements the Docker Engine protocol plugin.
package docker

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

// dockerComposeContent mirrors the docker-run recipe as an inline Compose file.
// It runs as root since Compose can't add the socket's GID dynamically.
const dockerComposeContent = `services:
  shellcn-agent:
    image: "{{.Image}}"
    container_name: shellcn-agent
    restart: unless-stopped
    network_mode: host
    user: "0:0"
    environment:
      SHELLCN_CONNECT_URL: "{{.GatewayConnectURL}}"
      SHELLCN_ENROLL_TOKEN: "{{.Token}}"
{{if .Insecure}}      SHELLCN_INSECURE: "1"
{{end}}    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
`

const dockerIconSVG = `<?xml version="1.0" encoding="UTF-8"?><svg id=Layer_1 version=1.1 viewBox="0 0 340 268"xmlns=http://www.w3.org/2000/svg xmlns:xlink=http://www.w3.org/1999/xlink><defs><style>.st0{fill:none}.st1{fill:#2560ff}.st2{clip-path:url(#clippath)}</style><clipPath id=clippath><rect class=st0 height=268 width=339.5 /></clipPath></defs><g class=st2><path class=st1 d=M334,110.1c-8.3-5.6-30.2-8-46.1-3.7-.9-15.8-9-29.2-24-40.8l-5.5-3.7-3.7,5.6c-7.2,11-10.3,25.7-9.2,39,.8,8.2,3.7,17.4,9.2,24.1-20.7,12-39.8,9.3-124.3,9.3H0c-.4,19.1,2.7,55.8,26,85.6,2.6,3.3,5.4,6.5,8.5,9.6,19,19,47.6,32.9,90.5,33,65.4,0,121.4-35.3,155.5-120.8,11.2.2,40.8,2,55.3-26,.4-.5,3.7-7.4,3.7-7.4l-5.5-3.7h0ZM85.2,92.7h-36.7v36.7h36.7v-36.7ZM132.6,92.7h-36.7v36.7h36.7v-36.7ZM179.9,92.7h-36.7v36.7h36.7v-36.7ZM227.3,92.7h-36.7v36.7h36.7v-36.7ZM37.8,92.7H1.1v36.7h36.7v-36.7ZM85.2,46.3h-36.7v36.7h36.7v-36.7ZM132.6,46.3h-36.7v36.7h36.7v-36.7ZM179.9,46.3h-36.7v36.7h36.7v-36.7ZM179.9,0h-36.7v36.7h36.7V0Z /></g></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.CurrentAPIVersion,
		Name:        "docker",
		Version:     "0.1.0",
		Title:       "Docker",
		Description: "Docker Engine cockpit with containers, images, volumes, networks, logs, exec, and events.",
		Icon:        plugin.Icon{Type: plugin.IconSVG, Value: dockerIconSVG},
		Category:    plugin.CategoryContainers,
		Config:      configSchema(),
		Capabilities: []plugin.Capability{
			"containers", "images", "volumes", "networks", "logs", "terminal", "events",
		},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentUnix, Address: "/var/run/docker.sock", Risk: plugin.RiskPrivileged, Forward: true},
			Install: []plugin.InstallArtifact{
				{
					Label:      "Docker",
					Kind:       "docker-run",
					ConnectURL: plugin.ArtifactConnectURL{LocalhostHost: "host.docker.internal"},
					// Host networking lets the agent reach container IPs on every Docker
					// network when proxying a web port.
					Template: "docker run --rm --name " + plugin.AgentBinary + " --network host " +
						"{{if .LocalhostHostRequired}}--add-host={{.LocalhostHost}}:host-gateway {{end}}" +
						`--group-add "$(stat -c '%g' /var/run/docker.sock)" ` +
						"-e SHELLCN_CONNECT_URL={{shellquote .ConnectURL}} " +
						"{{if .Insecure}}-e SHELLCN_INSECURE=1 {{end}}" +
						"-e SHELLCN_ENROLL_TOKEN={{shellquote .Token}} " +
						"-v {{shellquote \"/var/run/docker.sock:/var/run/docker.sock\"}} " +
						"{{shellquote .Image}}",
				},
				{
					Label:    "Docker Compose",
					Kind:     "docker-compose",
					Filename: "shellcn-agent.compose.yml",
					Content:  dockerComposeContent,
				},
			},
		},
		Layout:    plugin.LayoutSidebarTree,
		Tree:      tree(),
		Resources: resources(),
		Actions:   actions(),
		Streams: []plugin.Stream{
			{ID: "docker.container.logs", Kind: plugin.StreamLogs, RouteID: "docker.container.logs"},
			{ID: "docker.container.exec", Kind: plugin.StreamTerminal, RouteID: "docker.container.exec"},
			{ID: "docker.events.watch", Kind: plugin.StreamLogs, RouteID: "docker.events.watch"},
		},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2},
			StreamIDs: []string{"docker.container.exec"}, Authoritative: true,
		}},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return Routes()
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return Connect(ctx, cfg)
}

func configSchema() plugin.Schema {
	directOnly := plugin.Condition{AllOf: []plugin.Rule{{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)}}}
	directUnix := plugin.Condition{AllOf: []plugin.Rule{
		{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)},
		{Field: "endpoint_type", Op: plugin.OpEq, Value: "unix"},
	}}
	directTCP := plugin.Condition{AllOf: []plugin.Rule{
		{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)},
		{Field: "endpoint_type", Op: plugin.OpEq, Value: "tcp"},
	}}
	return plugin.Schema{Groups: []plugin.Group{{
		Name: "Endpoint",
		Fields: []plugin.Field{
			{Key: "endpoint_type", Label: "Endpoint", Type: plugin.FieldSelect, Required: true, Default: "unix", VisibleWhen: &directOnly, Options: []plugin.Option{
				{Label: "Unix socket", Value: "unix"},
				{Label: "TCP host", Value: "tcp"},
			}},
			{Key: "socket_path", Label: "Socket path", Type: plugin.FieldText, Required: true, Default: "/var/run/docker.sock", VisibleWhen: &directUnix},
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "docker.example.internal", VisibleWhen: &directTCP},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Required: true, Default: 2375, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, VisibleWhen: &directTCP},
		},
	}}}
}

func tree() []plugin.TreeGroup {
	return []plugin.TreeGroup{
		{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "layout-dashboard"}, Ref: dockerengine.OverviewRef()},
		{Key: "containers", Label: "Containers", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "box"}, ResourceKind: "container"},
		{Key: "compose", Label: "Compose", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "workflow"}, ResourceKind: "compose"},
		{Key: "images", Label: "Images", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "layers"}, ResourceKind: "image"},
		{Key: "volumes", Label: "Volumes", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "database"}, ResourceKind: "volume"},
		{Key: "networks", Label: "Networks", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "globe"}, ResourceKind: "network"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		overviewResource(),
		containerResource(),
		imageResource(),
		volumeResource(),
		networkResource(),
		composeResource(),
	}
}

func overviewResource() plugin.ResourceType {
	dash := plugin.DashboardConfig{Cells: []plugin.Panel{
		{Key: "stats", Label: "Environment", Type: plugin.PanelMetrics, Span: 2, Source: &plugin.DataSource{RouteID: "docker.overview.metrics", Method: plugin.MethodWS}, Config: dockerengine.OverviewMetricsConfig()},
		{Key: "containers", Label: "Containers", Type: plugin.PanelTable, Span: 2, Source: &plugin.DataSource{RouteID: "docker.containers.list"}, Config: plugin.TableConfig{Columns: containerColumns()}},
	}}
	return plugin.ResourceType{
		Kind: dockerengine.OverviewKind, Title: "Overview",
		List:    plugin.DataSource{RouteID: "docker.overview.list"},
		Columns: []plugin.Column{{Key: "name", Label: "Name"}},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Overview"},
			Tabs:   []plugin.Panel{{Key: "dashboard", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "layout-dashboard"}, Type: plugin.PanelDashboard, Config: dash}},
		},
	}
}

func containerColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "image", Label: "Image", Sortable: true},
		{Key: "state", Label: "State", Type: plugin.ColumnBadge, Sortable: true, Severities: dockerengine.StateSeverities()},
		{Key: "status", Label: "Status"},
		{Key: "ports", Label: "Ports"},
		{Key: "compose", Label: "Compose", Sortable: true},
		{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func serviceColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Service", Sortable: true},
		{Key: "image", Label: "Image", Sortable: true},
		{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "running", Label: "Running", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "ports", Label: "Ports"},
	}
}

func containerResource() plugin.ResourceType {
	columns := containerColumns()
	return plugin.ResourceType{
		Kind: "container", Title: "Containers",
		List:    plugin.DataSource{RouteID: "docker.containers.list"},
		Watch:   &plugin.DataSource{RouteID: "docker.events.watch", Method: plugin.MethodWS},
		Columns: columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{
				"docker.container.create",
				"docker.containers.prune",
			},
			Row:    []string{"docker.container.remove"},
			Detail: []string{"docker.container.open", "docker.container.start", "docker.container.stop", "docker.container.restart", "docker.container.pause", "docker.container.unpause", "docker.container.kill", "docker.container.rename", "docker.container.remove"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "state", Severities: dockerengine.StateSeverities()},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "info"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.container.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "terminal", Label: "Terminal", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "terminal"}, Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "docker.container.exec", Method: plugin.MethodWS, Params: map[string]string{"id": "${resource.uid}", "cols": "80", "rows": "24"}}, Config: plugin.TerminalConfig{Zoom: true, Search: true}},
				{Key: "logs", Label: "Logs", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "scroll-text"}, Type: plugin.PanelLogStream, Source: &plugin.DataSource{RouteID: "docker.container.logs", Method: plugin.MethodWS, Params: map[string]string{"id": "${resource.uid}", "tail": "200", "follow": "true", "timestamps": "true"}}},
				{Key: "inspect", Label: "Inspect", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "code"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.container.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "env", Label: "Env", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "list"}, Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "docker.container.env", Params: map[string]string{"id": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: []plugin.Column{{Key: "key", Label: "Key", Sortable: true}, {Key: "value", Label: "Value"}}}},
			},
		},
	}
}

func imageResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "tags", Label: "Tags"},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
	return plugin.ResourceType{
		Kind: "image", Title: "Images", List: plugin.DataSource{RouteID: "docker.images.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{"docker.image.pull", "docker.image.build", "docker.images.prune"},
			Row:     []string{"docker.image.remove"},
			Detail:  []string{"docker.image.tag", "docker.image.push", "docker.image.remove"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "info"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.image.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "inspect", Label: "Inspect", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "code"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.image.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func volumeResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "driver", Label: "Driver", Sortable: true},
		{Key: "scope", Label: "Scope", Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "refs", Label: "Refs", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "compose", Label: "Compose", Sortable: true},
	}
	return plugin.ResourceType{
		Kind: "volume", Title: "Volumes", List: plugin.DataSource{RouteID: "docker.volumes.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{"docker.volume.create", "docker.volumes.prune"},
			Row:     []string{"docker.volume.remove"},
			Detail:  []string{"docker.volume.remove"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "info"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.volume.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "inspect", Label: "Inspect", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "code"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.volume.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func networkResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "driver", Label: "Driver", Sortable: true},
		{Key: "scope", Label: "Scope", Sortable: true},
		{Key: "internal", Label: "Internal", Type: plugin.ColumnBool, Sortable: true},
		{Key: "attachable", Label: "Attachable", Type: plugin.ColumnBool, Sortable: true},
		{Key: "compose", Label: "Compose", Sortable: true},
	}
	return plugin.ResourceType{
		Kind: "network", Title: "Networks", List: plugin.DataSource{RouteID: "docker.networks.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{"docker.network.create", "docker.networks.prune"},
			Row:     []string{"docker.network.remove"},
			Detail:  []string{"docker.network.connect", "docker.network.disconnect", "docker.network.remove"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "info"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.network.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "inspect", Label: "Inspect", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "code"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.network.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func composeResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Project", Sortable: true},
		{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "running", Label: "Running", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "workingDir", Label: "Working dir"},
		{Key: "config", Label: "Config"},
	}
	return plugin.ResourceType{
		Kind: "compose", Title: "Compose", List: plugin.DataSource{RouteID: "docker.compose.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Row:    []string{"docker.compose.down"},
			Detail: []string{"docker.compose.up", "docker.compose.down"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "info"}, Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "docker.compose.overview", Params: map[string]string{"project": "${resource.uid}"}}},
				{Key: "containers", Label: "Containers", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "box"}, Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "docker.compose.containers", Params: map[string]string{"project": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: containerColumns()}},
				{Key: "services", Label: "Services", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "workflow"}, Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "docker.compose.services", Params: map[string]string{"project": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: serviceColumns()}},
			},
		},
	}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: "docker.container.create", Label: "New container", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "plus"}, RouteID: "docker.container.create"},
		{ID: "docker.container.open", Label: "Open", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "external-link"}, RouteID: "docker.container.open", Open: plugin.OpenURL, Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: dockerengine.WhenState("running")},
		{ID: "docker.container.start", Label: "Start", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "play"}, RouteID: "docker.container.start", Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: dockerengine.WhenState("created", "exited", "dead"), Group: "Lifecycle"},
		{ID: "docker.container.stop", Label: "Stop", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "square"}, RouteID: "docker.container.stop", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Stop this container?", EnabledWhen: dockerengine.WhenState("running", "paused", "restarting"), Group: "Lifecycle"},
		{ID: "docker.container.restart", Label: "Restart", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "refresh-cw"}, RouteID: "docker.container.restart", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Restart this container?", EnabledWhen: dockerengine.WhenState("running", "paused"), Group: "Lifecycle"},
		{ID: "docker.container.pause", Label: "Pause", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "pause"}, RouteID: "docker.container.pause", Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: dockerengine.WhenState("running"), Group: "Lifecycle"},
		{ID: "docker.container.unpause", Label: "Unpause", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "play"}, RouteID: "docker.container.unpause", Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: dockerengine.WhenState("paused"), Group: "Lifecycle"},
		{ID: "docker.container.kill", Label: "Kill", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "skull"}, RouteID: "docker.container.kill", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Send a kill signal to this container?", EnabledWhen: dockerengine.WhenState("running", "paused", "restarting"), Group: "Lifecycle"},
		{ID: "docker.container.rename", Label: "Rename", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "pencil"}, RouteID: "docker.container.rename", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "docker.container.remove", Label: "Remove", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "trash"}, RouteID: "docker.container.remove", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Remove this container and anonymous volumes?"},
		{ID: "docker.image.build", Label: "Build image", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "hammer"}, RouteID: "docker.image.build"},
		{ID: "docker.image.tag", Label: "Tag", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "tag"}, RouteID: "docker.image.tag", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "docker.image.push", Label: "Push", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "upload"}, RouteID: "docker.image.push", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "docker.image.remove", Label: "Remove", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "trash"}, RouteID: "docker.image.remove", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Remove this image?"},
		{ID: "docker.volume.remove", Label: "Remove", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "trash"}, RouteID: "docker.volume.remove", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Remove this volume?"},
		{ID: "docker.network.remove", Label: "Remove", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "trash"}, RouteID: "docker.network.remove", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Remove this network?"},
		{ID: "docker.image.pull", Label: "Pull image", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "download"}, RouteID: "docker.image.pull"},
		{ID: "docker.volume.create", Label: "Create volume", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "plus"}, RouteID: "docker.volume.create"},
		{ID: "docker.network.create", Label: "Create network", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "plus"}, RouteID: "docker.network.create"},
		{ID: "docker.network.connect", Label: "Connect container", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "link"}, RouteID: "docker.network.connect", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "docker.network.disconnect", Label: "Disconnect container", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "unlink"}, RouteID: "docker.network.disconnect", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "docker.compose.up", Label: "Up", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "play"}, RouteID: "docker.compose.up", Params: map[string]string{"project": "${resource.uid}"}, Confirm: true, ConfirmText: "Start all containers in this project?"},
		{ID: "docker.compose.down", Label: "Down", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "square"}, RouteID: "docker.compose.down", Params: map[string]string{"project": "${resource.uid}"}, Confirm: true, ConfirmText: "Stop and remove all containers in this project?"},
		{ID: "docker.containers.prune", Label: "Prune stopped", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "eraser"}, RouteID: "docker.containers.prune", Confirm: true, ConfirmText: "Remove all stopped containers?"},
		{ID: "docker.images.prune", Label: "Prune dangling", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "eraser"}, RouteID: "docker.images.prune", Confirm: true, ConfirmText: "Remove all dangling images?"},
		{ID: "docker.volumes.prune", Label: "Prune unused", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "eraser"}, RouteID: "docker.volumes.prune", Confirm: true, ConfirmText: "Remove all unused volumes?"},
		{ID: "docker.networks.prune", Label: "Prune unused", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "eraser"}, RouteID: "docker.networks.prune", Confirm: true, ConfirmText: "Remove all unused networks?"},
	}
}
