// Package swarm implements the Docker Swarm orchestration plugin. It speaks the
// same Docker daemon API as the docker plugin (via dockerengine) and adds the
// orchestration objects: services, stacks, nodes, and tasks.
package swarm

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

// stateSeverities colors a task/node state badge; availabilitySeverities colors a
// node's availability badge.
var (
	stateSeverities = map[string]plugin.Severity{
		"running": plugin.SeveritySuccess, "ready": plugin.SeveritySuccess,
		"complete": plugin.SeverityInfo,
		"new":      plugin.SeverityWarn, "pending": plugin.SeverityWarn, "assigned": plugin.SeverityWarn,
		"accepted": plugin.SeverityWarn, "preparing": plugin.SeverityWarn, "starting": plugin.SeverityWarn,
		"shutdown": plugin.SeveritySecondary,
		"failed":   plugin.SeverityDanger, "rejected": plugin.SeverityDanger, "orphaned": plugin.SeverityDanger,
		"down": plugin.SeverityDanger, "disconnected": plugin.SeverityDanger,
	}
	availabilitySeverities = map[string]plugin.Severity{
		"active": plugin.SeveritySuccess, "pause": plugin.SeverityWarn, "drain": plugin.SeverityWarn,
	}
)

const dockerIconSVG = `<?xml version="1.0" encoding="UTF-8"?><svg id=Layer_1 version=1.1 viewBox="0 0 340 268"xmlns=http://www.w3.org/2000/svg xmlns:xlink=http://www.w3.org/1999/xlink><defs><style>.st0{fill:none}.st1{fill:#2560ff}.st2{clip-path:url(#clippath)}</style><clipPath id=clippath><rect class=st0 height=268 width=339.5 /></clipPath></defs><g class=st2><path class=st1 d=M334,110.1c-8.3-5.6-30.2-8-46.1-3.7-.9-15.8-9-29.2-24-40.8l-5.5-3.7-3.7,5.6c-7.2,11-10.3,25.7-9.2,39,.8,8.2,3.7,17.4,9.2,24.1-20.7,12-39.8,9.3-124.3,9.3H0c-.4,19.1,2.7,55.8,26,85.6,2.6,3.3,5.4,6.5,8.5,9.6,19,19,47.6,32.9,90.5,33,65.4,0,121.4-35.3,155.5-120.8,11.2.2,40.8,2,55.3-26,.4-.5,3.7-7.4,3.7-7.4l-5.5-3.7h0ZM85.2,92.7h-36.7v36.7h36.7v-36.7ZM132.6,92.7h-36.7v36.7h36.7v-36.7ZM179.9,92.7h-36.7v36.7h36.7v-36.7ZM227.3,92.7h-36.7v36.7h36.7v-36.7ZM37.8,92.7H1.1v36.7h36.7v-36.7ZM85.2,46.3h-36.7v36.7h36.7v-36.7ZM132.6,46.3h-36.7v36.7h36.7v-36.7ZM179.9,46.3h-36.7v36.7h36.7v-36.7ZM179.9,0h-36.7v36.7h36.7V0Z /></g></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.CurrentAPIVersion,
		Name:        "swarm",
		Version:     "0.1.0",
		Title:       "Docker Swarm",
		Description: "Docker Swarm cockpit with services, stacks, nodes, tasks, and service logs.",
		Icon:        plugin.Icon{Type: plugin.IconSVG, Value: dockerIconSVG},
		Category:    plugin.CategoryContainers,
		Config:      configSchema(),
		Capabilities: []plugin.Capability{
			"services", "stacks", "nodes", "tasks", "logs",
		},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentUnix, Address: defaultSocket, Risk: plugin.RiskPrivileged, Forward: true},
			Install: []plugin.InstallArtifact{{
				Label:      "Docker Swarm",
				Kind:       "docker-run",
				ConnectURL: plugin.ArtifactConnectURL{LocalhostHost: "host.docker.internal"},
				Template: "docker run --rm --name " + plugin.AgentBinary + " " +
					"{{if .LocalhostHostRequired}}--add-host={{.LocalhostHost}}:host-gateway {{end}}" +
					`--group-add "$(stat -c '%g' /var/run/docker.sock)" ` +
					"-e SHELLCN_CONNECT_URL={{shellquote .ConnectURL}} " +
					"{{if .Insecure}}-e SHELLCN_INSECURE=1 {{end}}" +
					"-e SHELLCN_ENROLL_TOKEN={{shellquote .Token}} " +
					"-v {{shellquote \"/var/run/docker.sock:/var/run/docker.sock\"}} " +
					"{{shellquote .Image}}",
			}},
		},
		Layout:    plugin.LayoutSidebarTree,
		Tree:      tree(),
		Resources: resources(),
		Actions:   actions(),
		Streams: []plugin.Stream{
			{ID: "swarm.overview.metrics", Kind: plugin.StreamMetrics, RouteID: "swarm.overview.metrics"},
			{ID: "swarm.service.logs", Kind: plugin.StreamLogs, RouteID: "swarm.service.logs"},
		},
	}
}

func (p *Plugin) Routes() []plugin.Route { return Routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return Connect(ctx, cfg)
}

func icon(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

func tree() []plugin.TreeGroup {
	return []plugin.TreeGroup{
		{Key: "overview", Label: "Overview", Icon: icon("layout-dashboard"), Ref: dockerengine.OverviewRef()},
		{Key: "services", Label: "Services", Icon: icon("workflow"), Source: plugin.DataSource{RouteID: "swarm.services.tree"}, ResourceKind: "service"},
		{Key: "stacks", Label: "Stacks", Icon: icon("layers"), Source: plugin.DataSource{RouteID: "swarm.stacks.tree"}, ResourceKind: "stack"},
		{Key: "nodes", Label: "Nodes", Icon: icon("server"), Source: plugin.DataSource{RouteID: "swarm.nodes.tree"}, ResourceKind: "node"},
		{Key: "tasks", Label: "Tasks", Icon: icon("list-checks"), ResourceKind: "task"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		overviewResource(),
		serviceResource(),
		stackResource(),
		nodeResource(),
		taskResource(),
	}
}

func overviewResource() plugin.ResourceType {
	dash := plugin.DashboardConfig{Cells: []plugin.Panel{
		{Key: "stats", Label: "Cluster", Type: plugin.PanelMetrics, Span: 2, Source: &plugin.DataSource{RouteID: "swarm.overview.metrics", Method: plugin.MethodWS}, Config: overviewMetricsConfig()},
		{Key: "services", Label: "Services", Type: plugin.PanelTable, Span: 2, Source: &plugin.DataSource{RouteID: "swarm.services.list"}, Config: plugin.TableConfig{Columns: serviceColumns()}},
		{Key: "nodes", Label: "Nodes", Type: plugin.PanelTable, Span: 2, Source: &plugin.DataSource{RouteID: "swarm.nodes.list"}, Config: plugin.TableConfig{Columns: nodeResource().Columns}},
	}}
	return plugin.ResourceType{
		Kind: dockerengine.OverviewKind, Title: "Overview",
		List:    plugin.DataSource{RouteID: "swarm.overview.list"},
		Columns: []plugin.Column{{Key: "name", Label: "Name"}},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Overview"},
			Tabs:   []plugin.Panel{{Key: "dashboard", Label: "Overview", Icon: icon("layout-dashboard"), Type: plugin.PanelDashboard, Config: dash}},
		},
	}
}

func overviewMetricsConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{Stats: []plugin.MetricStat{
		{Key: "services", Label: "Services"},
		{Key: "nodes", Label: "Nodes"},
		{Key: "tasks", Label: "Tasks"},
		{Key: "stacks", Label: "Stacks"},
	}}
}

func serviceColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "mode", Label: "Mode", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "replicas", Label: "Replicas"},
		{Key: "image", Label: "Image", Sortable: true},
		{Key: "ports", Label: "Ports"},
		{Key: "stack", Label: "Stack", Sortable: true},
		{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func taskColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Task", Sortable: true},
		{Key: "node", Label: "Node", Sortable: true},
		{Key: "desiredState", Label: "Desired", Sortable: true},
		{Key: "state", Label: "State", Type: plugin.ColumnBadge, Sortable: true, Severities: stateSeverities},
		{Key: "image", Label: "Image"},
		{Key: "error", Label: "Error"},
	}
}

func serviceResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "service", Title: "Services",
		List:    plugin.DataSource{RouteID: "swarm.services.list"},
		Watch:   &plugin.DataSource{RouteID: "swarm.events.watch", Method: plugin.MethodWS},
		Columns: serviceColumns(),
		Actions: plugin.ResourceActions{
			Row:    []string{"swarm.service.remove"},
			Detail: []string{"swarm.service.open", "swarm.service.scale", "swarm.service.update", "swarm.service.rollback", "swarm.service.remove"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.service.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "tasks", Label: "Tasks", Icon: icon("list-checks"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "swarm.service.tasks", Params: map[string]string{"id": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: taskColumns()}},
				{Key: "logs", Label: "Logs", Icon: icon("scroll-text"), Type: plugin.PanelLogStream, Source: &plugin.DataSource{RouteID: "swarm.service.logs", Method: plugin.MethodWS, Params: map[string]string{"id": "${resource.uid}", "tail": "200", "follow": "true", "timestamps": "true"}}},
				{Key: "inspect", Label: "Inspect", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.service.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func stackResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Stack", Sortable: true},
		{Key: "services", Label: "Services", Type: plugin.ColumnNumber, Sortable: true},
	}
	return plugin.ResourceType{
		Kind: "stack", Title: "Stacks", List: plugin.DataSource{RouteID: "swarm.stacks.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{"swarm.stack.deploy"},
			Detail:  []string{"swarm.stack.deploy"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.stack.overview", Params: map[string]string{"stack": "${resource.uid}"}}},
				{Key: "services", Label: "Services", Icon: icon("workflow"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "swarm.stack.services", Params: map[string]string{"stack": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: serviceColumns()}},
			},
		},
	}
}

func nodeResource() plugin.ResourceType {
	columns := []plugin.Column{
		{Key: "name", Label: "Hostname", Sortable: true},
		{Key: "role", Label: "Role", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "availability", Label: "Availability", Type: plugin.ColumnBadge, Sortable: true, Severities: availabilitySeverities},
		{Key: "state", Label: "State", Type: plugin.ColumnBadge, Sortable: true, Severities: stateSeverities},
		{Key: "leader", Label: "Leader", Type: plugin.ColumnBool, Sortable: true},
		{Key: "engine", Label: "Engine", Sortable: true},
		{Key: "address", Label: "Address"},
	}
	return plugin.ResourceType{
		Kind: "node", Title: "Nodes", List: plugin.DataSource{RouteID: "swarm.nodes.list"}, Columns: columns,
		Actions: plugin.ResourceActions{
			Detail: []string{"swarm.node.update"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "state", Severities: stateSeverities},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.node.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "tasks", Label: "Tasks", Icon: icon("list-checks"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "swarm.node.tasks", Params: map[string]string{"id": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: taskColumns()}},
				{Key: "inspect", Label: "Inspect", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.node.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func taskResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "task", Title: "Tasks", List: plugin.DataSource{RouteID: "swarm.tasks.list"}, Columns: taskColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "state", Severities: stateSeverities},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.task.overview", Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "inspect", Label: "Inspect", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "swarm.task.inspect", Params: map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: "swarm.service.open", Label: "Open", Icon: icon("external-link"), RouteID: "swarm.service.open", Open: plugin.OpenURL, Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}}}},
		{ID: "swarm.service.scale", Label: "Scale", Icon: icon("move-vertical"), RouteID: "swarm.service.scale", Params: map[string]string{"id": "${resource.uid}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "mode", Op: plugin.OpEq, Value: "replicated"}}}, Group: "Manage"},
		{ID: "swarm.service.update", Label: "Update", Icon: icon("pencil"), RouteID: "swarm.service.update", Params: map[string]string{"id": "${resource.uid}"}, Group: "Manage"},
		{ID: "swarm.service.rollback", Label: "Rollback", Icon: icon("undo-2"), RouteID: "swarm.service.rollback", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Roll this service back to its previous spec?", Group: "Manage"},
		{ID: "swarm.service.remove", Label: "Remove", Icon: icon("trash"), RouteID: "swarm.service.remove", Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Remove this service?"},
		{ID: "swarm.node.update", Label: "Update", Icon: icon("settings"), RouteID: "swarm.node.update", Params: map[string]string{"id": "${resource.uid}"}},
		{ID: "swarm.stack.deploy", Label: "Deploy stack", Icon: icon("upload"), RouteID: "swarm.stack.deploy"},
	}
}
