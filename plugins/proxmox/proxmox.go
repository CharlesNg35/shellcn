// Package proxmox implements the Proxmox VE protocol plugin: a deep node →
// guests/storage tree, VM/LXC/node detail views, live metrics, noVNC and xterm
// consoles, snapshots, backups, and lifecycle actions — all over the PVE REST API
// (with the console websocket bridged through the gateway transport).
package proxmox

import (
	"context"

	"github.com/charlesng/shellcn/internal/plugin"
)

const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" xml:space="preserve" viewBox="0 34 512 444"><path d="M137.9 34.1c-10.5 0-19.7 1.9-28.5 5.7-8.6 3.8-16.2 8.9-22.9 15.6l170 186.4L426.1 55.3c-6.7-6.7-14.3-11.8-23.4-15.6-8.3-3.8-18-5.7-28-5.7-10.5 0-20.5 2.2-29.4 6.2-9.2 4-16.7 10-23.7 17l-65.2 72.2-66-72.2c-6.7-7-14.3-12.9-23.7-17-8.3-4-18.3-6.1-28.8-6.1M256.4 270l-170 186.7c6.7 6.5 14.3 11.8 22.9 15.6 8.9 3.8 18.1 5.7 28 5.7 11 0 20.5-2.4 29.4-6.2 9.4-4.3 17.5-10 24.2-17l65.5-72.2 65.4 72.2c6.7 7 14.3 12.7 23.4 17 8.9 3.8 18.6 6.2 29.4 6.2 10 0 19.7-1.9 28-5.7 9.2-3.8 16.7-9.2 23.4-15.6z" style="fill-rule:evenodd;clip-rule:evenodd"/><path d="M56 90.1c-10.8.3-21.3 2.4-30.7 6.5-9.7 4-18 9.7-25.3 16.7L129.8 256 0 398.5c7.3 7.3 15.6 12.9 25.3 17.2 9.4 4.3 19.9 6.2 30.7 6.7 11.6-.5 22.4-2.4 32.3-7.3q15-6.9 25.8-18.6l128-140.5-127.9-140.3c-7.8-7.5-16.2-13.7-26.1-18.6-10-4.6-20.5-6.7-32.1-7m399.7 0c-11.6.3-21.8 2.4-31.8 7-10 4.8-18.6 11-26.1 18.6L270.4 256l127.4 140.6q11.25 11.7 26.1 18.6c10 4.8 20.2 6.7 31.8 7.3 11.6-.5 21.5-2.4 31-6.7 10.2-4.3 18-10 25.3-17.2L382.5 256 512 113.3c-7.3-7-15.1-12.7-25.3-16.7-9.4-4.1-19.4-6.2-31-6.5" style="fill-rule:evenodd;clip-rule:evenodd;fill:#e57000"/></svg>`

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "proxmox",
		Version:             "0.1.0",
		Title:               "Proxmox VE",
		Description:         "Proxmox Virtual Environment cockpit: nodes, VMs, containers, storage, consoles, snapshots, and backups.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: iconSVG},
		Category:            plugin.CategoryVirtualization,
		Config:              configSchema("proxmox"),
		Capabilities:        []plugin.Capability{"nodes", "vms", "containers", "storage", "remote_desktop", "terminal", "snapshots", "backups"},
		CredentialKinds:     credentialKinds(),
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSidebarTree,
		Tree: []plugin.TreeGroup{
			{Key: "nodes", Label: "Nodes", Icon: icon("server"), Source: plugin.DataSource{RouteID: "proxmox.tree.nodes"}, ResourceKind: "node"},
			{Key: "storage", Label: "Storage", Icon: icon("database"), Source: plugin.DataSource{RouteID: "proxmox.tree.storage"}, ResourceKind: "storage"},
		},
		Resources: resources(),
		Actions:   actions(),
		Streams: []plugin.Stream{
			{ID: "proxmox.qemu.console", Kind: plugin.StreamDesktop, RouteID: "proxmox.qemu.console"},
			{ID: "proxmox.lxc.console", Kind: plugin.StreamTerminal, RouteID: "proxmox.lxc.console"},
			{ID: "proxmox.node.shell", Kind: plugin.StreamTerminal, RouteID: "proxmox.node.shell"},
			{ID: "proxmox.qemu.metrics", Kind: plugin.StreamMetrics, RouteID: "proxmox.qemu.metrics"},
			{ID: "proxmox.lxc.metrics", Kind: plugin.StreamMetrics, RouteID: "proxmox.lxc.metrics"},
			{ID: "proxmox.node.metrics", Kind: plugin.StreamMetrics, RouteID: "proxmox.node.metrics"},
		},
		Recording: []plugin.RecordingCapability{
			{Class: plugin.RecordingDesktop, Formats: []plugin.RecordingFormat{plugin.FormatWebMCanvas}, StreamIDs: []string{"proxmox.qemu.console"}},
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"proxmox.lxc.console", "proxmox.node.shell"}},
		},
	}
}

func (p *Plugin) Routes() []plugin.Route { return Routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return connect(ctx, cfg)
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{qemuResource(), lxcResource(), nodeResource(), storageResource()}
}

func guestColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "vmid", Label: "VMID", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "node", Label: "Node", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "cpu", Label: "CPU %", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "mem", Label: "Memory", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "tags", Label: "Tags"},
	}
}

func snapshotColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "description", Label: "Description"},
		{Key: "parent", Label: "Parent"},
		{Key: "snaptime", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func backupColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Volume", Sortable: true},
		{Key: "storage", Label: "Storage", Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "format", Label: "Format"},
		{Key: "ctime", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "notes", Label: "Notes"},
	}
}

func qemuResource() plugin.ResourceType {
	cols := guestColumns()
	lifecycle := []string{"act.qemu.start", "act.qemu.shutdown", "act.qemu.reboot", "act.qemu.stop", "act.qemu.suspend", "act.qemu.resume", "act.qemu.migrate", "act.qemu.snapshot.create", "act.qemu.backup"}
	return plugin.ResourceType{
		Kind: "qemu", Title: "Virtual Machines",
		List: plugin.DataSource{RouteID: "proxmox.qemu.list"}, Columns: cols,
		ActionIDs: lifecycle,
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", ActionIDs: lifecycle},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("activity"), Panel: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.qemu.metrics", Method: plugin.MethodWS, Params: guestParams()}, Config: cpuMemMetrics()},
				{Key: "console", Label: "Console", Icon: icon("monitor"), Panel: plugin.PanelRemoteDesktop, Source: &plugin.DataSource{RouteID: "proxmox.qemu.console", Method: plugin.MethodWS, Params: guestParams()}, Config: plugin.RemoteDesktopConfig{Resize: true, Clipboard: true}.Map()},
				{Key: "snapshots", Label: "Snapshots", Icon: icon("camera"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.qemu.snapshots", Params: guestParams()}, Config: plugin.TableConfig{Columns: snapshotColumns(), RowActionIDs: []string{"act.qemu.snapshot.rollback", "act.qemu.snapshot.delete"}}.Map()},
				{Key: "backups", Label: "Backups", Icon: icon("save"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.qemu.backups", Params: guestParams()}, Config: plugin.TableConfig{Columns: backupColumns(), RowActionIDs: []string{"act.backup.delete"}}.Map()},
				{Key: "hardware", Label: "Hardware", Icon: icon("cpu"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "proxmox.qemu.config", Params: guestParams()}},
			},
		},
	}
}

func lxcResource() plugin.ResourceType {
	cols := guestColumns()
	lifecycle := []string{"act.lxc.start", "act.lxc.shutdown", "act.lxc.reboot", "act.lxc.stop", "act.lxc.migrate", "act.lxc.snapshot.create", "act.lxc.backup"}
	return plugin.ResourceType{
		Kind: "lxc", Title: "Containers",
		List: plugin.DataSource{RouteID: "proxmox.lxc.list"}, Columns: cols,
		ActionIDs: lifecycle,
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", ActionIDs: lifecycle},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("activity"), Panel: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.lxc.metrics", Method: plugin.MethodWS, Params: guestParams()}, Config: cpuMemMetrics()},
				{Key: "console", Label: "Console", Icon: icon("terminal"), Panel: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "proxmox.lxc.console", Method: plugin.MethodWS, Params: guestParams()}, Config: plugin.TerminalConfig{Zoom: true, Search: true}.Map()},
				{Key: "snapshots", Label: "Snapshots", Icon: icon("camera"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.lxc.snapshots", Params: guestParams()}, Config: plugin.TableConfig{Columns: snapshotColumns(), RowActionIDs: []string{"act.lxc.snapshot.rollback", "act.lxc.snapshot.delete"}}.Map()},
				{Key: "backups", Label: "Backups", Icon: icon("save"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.lxc.backups", Params: guestParams()}, Config: plugin.TableConfig{Columns: backupColumns(), RowActionIDs: []string{"act.backup.delete"}}.Map()},
				{Key: "config", Label: "Config", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "proxmox.lxc.config", Params: guestParams()}},
			},
		},
	}
}

func nodeResource() plugin.ResourceType {
	cols := []plugin.Column{
		{Key: "name", Label: "Node", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "cpu", Label: "CPU %", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "mem", Label: "Memory", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "maxmem", Label: "Total", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber, Sortable: true},
	}
	nodeParam := map[string]string{"node": "${resource.uid}"}
	return plugin.ResourceType{
		Kind: "node", Title: "Nodes",
		List: plugin.DataSource{RouteID: "proxmox.node.list"}, Columns: cols,
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status"},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("activity"), Panel: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.node.metrics", Method: plugin.MethodWS, Params: nodeParam}, Config: cpuMemMetrics()},
				{Key: "shell", Label: "Shell", Icon: icon("terminal"), Panel: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "proxmox.node.shell", Method: plugin.MethodWS, Params: nodeParam}, Config: plugin.TerminalConfig{Zoom: true, Search: true}.Map()},
				{Key: "storage", Label: "Storage", Icon: icon("database"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.node.storage", Params: nodeParam}, Config: plugin.TableConfig{Columns: storageColumns()}.Map()},
				{Key: "tasks", Label: "Tasks", Icon: icon("list"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.node.tasks", Params: nodeParam}, Config: plugin.TableConfig{Columns: taskColumns()}.Map()},
			},
		},
	}
}

func storageResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "storage", Title: "Storage",
		List: plugin.DataSource{RouteID: "proxmox.storage.list"}, Columns: storageColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status"},
			Tabs: []plugin.Tab{
				{Key: "content", Label: "Content", Icon: icon("hard-drive"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.storage.content", Params: map[string]string{"node": "${resource.namespace}", "storage": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: contentColumns(), RowActionIDs: []string{"act.backup.delete"}}.Map()},
			},
		},
	}
}

func storageColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Storage", Sortable: true},
		{Key: "node", Label: "Node", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "content", Label: "Content"},
		{Key: "used", Label: "Used", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "total", Label: "Total", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge},
	}
}

func contentColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Volume", Sortable: true},
		{Key: "content", Label: "Type", Sortable: true},
		{Key: "format", Label: "Format"},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "vmid", Label: "VMID"},
		{Key: "ctime", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func taskColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Type", Sortable: true},
		{Key: "id", Label: "ID"},
		{Key: "user", Label: "User", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge},
		{Key: "starttime", Label: "Started", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func guestParams() map[string]string {
	return map[string]string{"node": "${resource.namespace}", "vmid": "${resource.uid}"}
}

// cpuMemMetrics declares the CPU/Memory gauges + time-series for the metrics
// panel. Nodes, VMs, and containers all stream `{cpu, mem}` percentage frames.
func cpuMemMetrics() map[string]any {
	return plugin.MetricsConfig{
		Gauges: []plugin.MetricGauge{
			{Key: "cpu", Label: "CPU", Unit: "%", Max: 100},
			{Key: "mem", Label: "Memory", Unit: "%", Max: 100},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU", Unit: "%"},
			{Key: "mem", Label: "Memory", Unit: "%"},
		},
	}.Map()
}

func actions() []plugin.Action {
	acts := append(lifecycleActions("qemu"), lifecycleActions("lxc")...)
	return append(acts, plugin.Action{
		ID: "act.backup.delete", Label: "Delete", Icon: icon("trash"), RouteID: "proxmox.backup.delete",
		Params:  map[string]string{"node": "${resource.namespace}", "storage": "${resource.name}", "volume": "${resource.uid}"},
		Confirm: true, ConfirmText: "Delete this volume? This cannot be undone.",
	})
}

func lifecycleActions(kind string) []plugin.Action {
	gp := guestParams()
	acts := []plugin.Action{
		{ID: "act." + kind + ".start", Label: "Start", Icon: icon("play"), RouteID: "proxmox." + kind + ".start", Params: gp},
		{ID: "act." + kind + ".shutdown", Label: "Shutdown", Icon: icon("power"), RouteID: "proxmox." + kind + ".shutdown", Params: gp, Confirm: true, ConfirmText: "Gracefully shut down this guest?"},
		{ID: "act." + kind + ".reboot", Label: "Reboot", Icon: icon("rotate-cw"), RouteID: "proxmox." + kind + ".reboot", Params: gp, Confirm: true, ConfirmText: "Reboot this guest?"},
		{ID: "act." + kind + ".stop", Label: "Stop", Icon: icon("square"), RouteID: "proxmox." + kind + ".stop", Params: gp, Confirm: true, ConfirmText: "Force stop this guest? Unsaved state is lost."},
		{ID: "act." + kind + ".migrate", Label: "Migrate", Icon: icon("route"), RouteID: "proxmox." + kind + ".migrate", Params: gp},
		{ID: "act." + kind + ".snapshot.create", Label: "Snapshot", Icon: icon("camera"), RouteID: "proxmox." + kind + ".snapshot.create", Params: gp, OnSuccess: &plugin.ActionSuccess{SelectTab: "snapshots"}},
		{ID: "act." + kind + ".backup", Label: "Backup", Icon: icon("save"), RouteID: "proxmox." + kind + ".backup", Params: gp, OnSuccess: &plugin.ActionSuccess{SelectTab: "backups"}},
	}
	if kind == "qemu" {
		acts = append(acts,
			plugin.Action{ID: "act.qemu.suspend", Label: "Suspend", Icon: icon("power-off"), RouteID: "proxmox.qemu.suspend", Params: gp, Confirm: true, ConfirmText: "Suspend this VM to disk?"},
			plugin.Action{ID: "act.qemu.resume", Label: "Resume", Icon: icon("play"), RouteID: "proxmox.qemu.resume", Params: gp},
		)
	}
	snapParams := map[string]string{"node": "${resource.namespace}", "vmid": "${resource.name}", "snapname": "${resource.uid}"}
	return append(acts,
		plugin.Action{ID: "act." + kind + ".snapshot.rollback", Label: "Rollback", Icon: icon("rotate-cw"), RouteID: "proxmox." + kind + ".snapshot.rollback", Params: snapParams, Confirm: true, ConfirmText: "Roll back to this snapshot? Current state is lost."},
		plugin.Action{ID: "act." + kind + ".snapshot.delete", Label: "Delete", Icon: icon("trash"), RouteID: "proxmox." + kind + ".snapshot.delete", Params: snapParams, Confirm: true, ConfirmText: "Delete this snapshot?"},
	)
}
