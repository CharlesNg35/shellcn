// Package proxmox implements the Proxmox VE protocol plugin: a deep node →
// guests/storage tree, VM/LXC/node detail views, live metrics, noVNC and xterm
// consoles, snapshots, backups, and lifecycle actions — all over the PVE REST API
// (with the console websocket bridged through the gateway transport).
package proxmox

import (
	"context"
	"fmt"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" xml:space="preserve" viewBox="0 34 512 444"><path d="M137.9 34.1c-10.5 0-19.7 1.9-28.5 5.7-8.6 3.8-16.2 8.9-22.9 15.6l170 186.4L426.1 55.3c-6.7-6.7-14.3-11.8-23.4-15.6-8.3-3.8-18-5.7-28-5.7-10.5 0-20.5 2.2-29.4 6.2-9.2 4-16.7 10-23.7 17l-65.2 72.2-66-72.2c-6.7-7-14.3-12.9-23.7-17-8.3-4-18.3-6.1-28.8-6.1M256.4 270l-170 186.7c6.7 6.5 14.3 11.8 22.9 15.6 8.9 3.8 18.1 5.7 28 5.7 11 0 20.5-2.4 29.4-6.2 9.4-4.3 17.5-10 24.2-17l65.5-72.2 65.4 72.2c6.7 7 14.3 12.7 23.4 17 8.9 3.8 18.6 6.2 29.4 6.2 10 0 19.7-1.9 28-5.7 9.2-3.8 16.7-9.2 23.4-15.6z" style="fill-rule:evenodd;clip-rule:evenodd"/><path d="M56 90.1c-10.8.3-21.3 2.4-30.7 6.5-9.7 4-18 9.7-25.3 16.7L129.8 256 0 398.5c7.3 7.3 15.6 12.9 25.3 17.2 9.4 4.3 19.9 6.2 30.7 6.7 11.6-.5 22.4-2.4 32.3-7.3q15-6.9 25.8-18.6l128-140.5-127.9-140.3c-7.8-7.5-16.2-13.7-26.1-18.6-10-4.6-20.5-6.7-32.1-7m399.7 0c-11.6.3-21.8 2.4-31.8 7-10 4.8-18.6 11-26.1 18.6L270.4 256l127.4 140.6q11.25 11.7 26.1 18.6c10 4.8 20.2 6.7 31.8 7.3 11.6-.5 21.5-2.4 31-6.7 10.2-4.3 18-10 25.3-17.2L382.5 256 512 113.3c-7.3-7-15.1-12.7-25.3-16.7-9.4-4.1-19.4-6.2-31-6.5" style="fill-rule:evenodd;clip-rule:evenodd;fill:#e57000"/></svg>`

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

// statusSeverities colors guest/node/task/storage status badges by value.
var statusSeverities = map[string]plugin.Severity{
	"running": plugin.SeveritySuccess, "online": plugin.SeveritySuccess, "ok": plugin.SeveritySuccess, "available": plugin.SeveritySuccess, "active": plugin.SeveritySuccess,
	"OK":      plugin.SeveritySuccess,
	"stopped": plugin.SeveritySecondary, "offline": plugin.SeveritySecondary, "disabled": plugin.SeveritySecondary,
	"paused": plugin.SeverityWarn, "unknown": plugin.SeverityWarn, "WARNINGS": plugin.SeverityWarn,
	"error": plugin.SeverityDanger, "ERROR": plugin.SeverityDanger,
}

var templateSeverities = map[string]plugin.Severity{
	"template": plugin.SeverityInfo,
	"instance": plugin.SeveritySecondary,
}

func oneDecimal() *int { return ptr(1) }

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
			{Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2}, StreamIDs: []string{"proxmox.lxc.console", "proxmox.node.shell"}, Authoritative: true},
		},
	}
}

func (p *Plugin) Routes() []plugin.Route { return Routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return connect(ctx, cfg)
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{guestResource(), qemuResource(), lxcResource(), nodeResource(), storageResource(), taskResource()}
}

func guestColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "kindIcon", Label: "", Type: plugin.ColumnIcon, Width: "3rem"},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "mode", Label: "Mode", Type: plugin.ColumnBadge, Sortable: true, Severities: templateSeverities},
		{Key: "vmid", Label: "VMID", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "node", Label: "Node", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true, Severities: statusSeverities},
		{Key: "cpu", Label: "CPU", Type: plugin.ColumnPercent, Sortable: true, Precision: oneDecimal()},
		{Key: "mem", Label: "Memory used", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "maxmem", Label: "Memory limit", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "tags", Label: "Tags"},
	}
}

func guestResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "guest",
		Title:   "Guests",
		List:    plugin.DataSource{RouteID: "proxmox.guest.list"},
		Columns: guestColumns(),
	}
}

func snapshotColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "description", Label: "Description"},
		{Key: "vmstate", Label: "RAM state", Type: plugin.ColumnBool},
		{Key: "snaptime", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func backupColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Volume", Sortable: true},
		{Key: "guestType", Label: "Guest", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "storage", Label: "Storage", Sortable: true},
		{Key: "protected", Label: "Protected", Type: plugin.ColumnBool, Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "format", Label: "Format"},
		{Key: "ctime", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "notes", Label: "Notes"},
	}
}

func qemuResource() plugin.ResourceType {
	cols := guestColumns()
	lifecycle := []string{"act.qemu.start", "act.qemu.shutdown", "act.qemu.reboot", "act.qemu.stop", "act.qemu.suspend", "act.qemu.resume", "act.qemu.migrate", "act.qemu.clone", "act.qemu.resize", "act.qemu.snapshot.create", "act.qemu.backup", "act.qemu.destroy"}
	row := []string{"act.qemu.destroy"}
	return plugin.ResourceType{
		Kind: "qemu", Title: "Virtual Machines",
		List: plugin.DataSource{RouteID: "proxmox.qemu.list"}, Columns: cols,
		Actions: plugin.ResourceActions{Detail: lifecycle, Row: row},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "summary", Label: "Summary", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "proxmox.qemu.overview", Params: guestParams()}, Config: guestOverviewConfig()},
				{Key: "metrics", Label: "Metrics", Icon: icon("activity"), Type: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.qemu.metrics", Method: plugin.MethodWS, Params: guestParams()}, Config: cpuMemMetrics(), VisibleWhen: instanceOnly()},
				{Key: "console", Label: "Console", Icon: icon("monitor"), Type: plugin.PanelRemoteDesktop, Source: &plugin.DataSource{RouteID: "proxmox.qemu.console", Method: plugin.MethodWS, Params: guestParams()}, Config: plugin.RemoteDesktopConfig{Resize: true}, VisibleWhen: instanceOnly()},
				{Key: "snapshots", Label: "Snapshots", Icon: icon("camera"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.qemu.snapshots", Params: guestParams()}, Config: plugin.TableConfig{Columns: snapshotColumns(), RowActionIDs: []string{"act.qemu.snapshot.rollback", "act.qemu.snapshot.delete"}, EmptyText: "No snapshots for this VM."}, VisibleWhen: instanceOnly()},
				{Key: "backups", Label: "Backups", Icon: icon("archive"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.qemu.backups", Params: guestParams()}, Config: plugin.TableConfig{Columns: backupColumns(), RowActionIDs: []string{"act.qemu.backup.restore", "act.backup.delete"}, RowClick: plugin.RowClickSelect, EmptyText: "No backup archives found for this VM."}},
				{Key: "hardware", Label: "Hardware", Icon: icon("cpu"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "proxmox.qemu.config", Params: guestParams()}, Config: qemuHardwareConfig()},
			},
		},
	}
}

func lxcResource() plugin.ResourceType {
	cols := guestColumns()
	lifecycle := []string{"act.lxc.start", "act.lxc.shutdown", "act.lxc.reboot", "act.lxc.stop", "act.lxc.migrate", "act.lxc.clone", "act.lxc.snapshot.create", "act.lxc.backup", "act.lxc.destroy"}
	row := []string{"act.lxc.destroy"}
	return plugin.ResourceType{
		Kind: "lxc", Title: "Containers",
		List: plugin.DataSource{RouteID: "proxmox.lxc.list"}, Columns: cols,
		Actions: plugin.ResourceActions{Detail: lifecycle, Row: row},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "summary", Label: "Summary", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "proxmox.lxc.overview", Params: guestParams()}, Config: guestOverviewConfig()},
				{Key: "metrics", Label: "Metrics", Icon: icon("activity"), Type: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.lxc.metrics", Method: plugin.MethodWS, Params: guestParams()}, Config: cpuMemMetrics(), VisibleWhen: instanceOnly()},
				{Key: "console", Label: "Console", Icon: icon("terminal"), Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "proxmox.lxc.console", Method: plugin.MethodWS, Params: guestParams()}, Config: plugin.TerminalConfig{Zoom: true, Search: true}, VisibleWhen: instanceOnly()},
				{Key: "snapshots", Label: "Snapshots", Icon: icon("camera"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.lxc.snapshots", Params: guestParams()}, Config: plugin.TableConfig{Columns: snapshotColumns(), RowActionIDs: []string{"act.lxc.snapshot.rollback", "act.lxc.snapshot.delete"}, EmptyText: "No snapshots for this container."}, VisibleWhen: instanceOnly()},
				{Key: "backups", Label: "Backups", Icon: icon("archive"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.lxc.backups", Params: guestParams()}, Config: plugin.TableConfig{Columns: backupColumns(), RowActionIDs: []string{"act.lxc.backup.restore", "act.backup.delete"}, RowClick: plugin.RowClickSelect, EmptyText: "No backup archives found for this container."}},
				{Key: "config", Label: "Config", Icon: icon("code"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "proxmox.lxc.config", Params: guestParams()}, Config: lxcConfigDetail()},
			},
		},
	}
}

func nodeResource() plugin.ResourceType {
	cols := []plugin.Column{
		{Key: "name", Label: "Node", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true, Severities: statusSeverities},
		{Key: "cpu", Label: "CPU", Type: plugin.ColumnPercent, Sortable: true, Precision: oneDecimal()},
		{Key: "mem", Label: "Memory", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "maxmem", Label: "Total", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber, Sortable: true},
	}
	nodeParam := map[string]string{"node": "${resource.uid}"}
	return plugin.ResourceType{
		Kind: "node", Title: "Nodes",
		List: plugin.DataSource{RouteID: "proxmox.node.list"}, Columns: cols,
		Actions: plugin.ResourceActions{Detail: []string{"act.node.power"}},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("activity"), Type: plugin.PanelMetrics, Source: &plugin.DataSource{RouteID: "proxmox.node.metrics", Method: plugin.MethodWS, Params: nodeParam}, Config: cpuMemMetrics()},
				{Key: "shell", Label: "Shell", Icon: icon("terminal"), Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "proxmox.node.shell", Method: plugin.MethodWS, Params: nodeParam}, Config: plugin.TerminalConfig{Zoom: true, Search: true}},
				{Key: "storage", Label: "Storage", Icon: icon("database"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.node.storage", Params: nodeParam}, Config: plugin.TableConfig{Columns: storageColumns(), EmptyText: "No storage is available on this node."}},
				{Key: "tasks", Label: "Task History", Icon: icon("list"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.node.tasks", Params: nodeParam}, Config: plugin.TableConfig{Columns: taskColumns(), RowActionIDs: []string{"act.task.stop"}, RowClick: plugin.RowClickDetail, DefaultSort: &plugin.SortKey{Field: "starttime", Desc: true}, EmptyText: "No recent tasks on this node."}},
			},
		},
	}
}

func storageResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "storage", Title: "Storage",
		List: plugin.DataSource{RouteID: "proxmox.storage.list"}, Columns: storageColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "content", Label: "Content", Icon: icon("hard-drive"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.storage.content", Params: map[string]string{"node": "${resource.namespace}", "storage": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: contentColumns(), RowActionIDs: []string{"act.qemu.backup.restore", "act.lxc.backup.restore", "act.backup.delete"}, RowClick: plugin.RowClickSelect, EmptyText: "This storage has no content visible to this connection."}},
			},
		},
	}
}

func taskResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "task",
		Title:   "Tasks",
		List:    plugin.DataSource{RouteID: "proxmox.task.list"},
		Columns: taskColumns(),
		Actions: plugin.ResourceActions{Detail: []string{"act.task.stop"}},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.uid}", StatusField: "status", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "status", Label: "Status", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "proxmox.task.status", Params: map[string]string{"node": "${resource.namespace}", "upid": "${resource.uid}"}}, Config: taskStatusConfig()},
				{Key: "log", Label: "Log", Icon: icon("scroll-text"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "proxmox.task.log", Params: map[string]string{"node": "${resource.namespace}", "upid": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: []plugin.Column{{Key: "n", Label: "#", Type: plugin.ColumnNumber, Width: "5rem"}, {Key: "t", Label: "Message"}}, EmptyText: "This task has no log lines."}},
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
		{Key: "usedPct", Label: "Used", Type: plugin.ColumnPercent, Sortable: true, Precision: oneDecimal()},
		{Key: "used", Label: "Used", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "total", Label: "Total", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: statusSeverities},
	}
}

func contentColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Volume", Sortable: true},
		{Key: "content", Label: "Type", Sortable: true},
		{Key: "format", Label: "Format"},
		{Key: "protected", Label: "Protected", Type: plugin.ColumnBool, Sortable: true},
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
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: statusSeverities},
		{Key: "exitstatus", Label: "Exit status", Type: plugin.ColumnBadge, Severities: statusSeverities},
		{Key: "starttime", Label: "Started", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "endtime", Label: "Ended", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func guestParams() map[string]string {
	return map[string]string{"node": "${resource.namespace}", "vmid": "${resource.uid}"}
}

func instanceOnly() *plugin.Condition {
	return &plugin.Condition{AllOf: []plugin.Rule{{Field: "template", Op: plugin.OpNeq, Value: true}}}
}

func guestOverviewConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "Identity", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "Name"},
				{Key: "vmid", Label: "VMID"},
				{Key: "node", Label: "Node"},
				{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: statusSeverities},
				{Key: "template", Label: "Template", Type: plugin.ColumnBool},
				{Key: "tags", Label: "Tags"},
			}},
			{Title: "Runtime", Fields: []plugin.ObjectDetailField{
				{Key: "cpu", Label: "CPU usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "cpu", TotalKey: "cpuTotal", TotalType: plugin.ColumnNumber, TotalLabel: "of", Unit: "CPU(s)", WarnAt: 75, CriticalAt: 90}},
				{Key: "memPct", Label: "Memory usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "memPct", UsedKey: "mem", TotalKey: "maxmem", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 95}},
				{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber},
				{Key: "lock", Label: "Lock"},
				{Key: "ha", Label: "HA state"},
			}},
			{Title: "Allocation", Fields: []plugin.ObjectDetailField{
				{Key: "cores", Label: "Cores", Type: plugin.ColumnNumber},
				{Key: "sockets", Label: "Sockets", Type: plugin.ColumnNumber},
				{Key: "memoryConfigured", Label: "Configured maximum RAM", Type: plugin.ColumnBytes},
				{Key: "memoryMinimum", Label: "Minimum RAM (balloon)", Type: plugin.ColumnBytes},
				{Key: "memoryCurrent", Label: "Current online RAM", Type: plugin.ColumnBytes},
				{Key: "ostype", Label: "OS type"},
			}},
		},
		RawToggle: true,
	}
}

func qemuHardwareConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "CPU and memory", Fields: []plugin.ObjectDetailField{
				{Key: "cores", Label: "Cores", Type: plugin.ColumnNumber},
				{Key: "sockets", Label: "Sockets", Type: plugin.ColumnNumber},
				{Key: "cpu", Label: "CPU type"},
				{Key: "memory", Label: "Maximum RAM"},
				{Key: "balloon", Label: "Minimum RAM (balloon)"},
				{Key: "shares", Label: "Balloon shares", Type: plugin.ColumnNumber},
				{Key: "numa", Label: "NUMA"},
			}},
			{Title: "Boot and firmware", Fields: []plugin.ObjectDetailField{
				{Key: "bios", Label: "BIOS"},
				{Key: "machine", Label: "Machine"},
				{Key: "ostype", Label: "OS type"},
				{Key: "boot", Label: "Boot order"},
				{Key: "scsihw", Label: "SCSI controller"},
			}},
			{Title: "Disks", Fields: diskFields()},
			{Title: "Network", Fields: netFields()},
		},
		RawToggle: true,
	}
}

func lxcConfigDetail() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "Resources", Fields: []plugin.ObjectDetailField{
				{Key: "cores", Label: "Cores", Type: plugin.ColumnNumber},
				{Key: "memory", Label: "Memory"},
				{Key: "swap", Label: "Swap"},
				{Key: "rootfs", Label: "Root disk"},
			}},
			{Title: "Container", Fields: []plugin.ObjectDetailField{
				{Key: "hostname", Label: "Hostname"},
				{Key: "ostype", Label: "OS type"},
				{Key: "arch", Label: "Architecture"},
				{Key: "unprivileged", Label: "Unprivileged", Type: plugin.ColumnBool},
				{Key: "features", Label: "Features"},
				{Key: "tags", Label: "Tags"},
			}},
			{Title: "Network", Fields: netFields()},
			{Title: "Mount points", Fields: mountFields()},
		},
		RawToggle: true,
	}
}

func taskStatusConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{{Title: "Task", Fields: []plugin.ObjectDetailField{
			{Key: "upid", Label: "UPID", Copy: true},
			{Key: "type", Label: "Type"},
			{Key: "id", Label: "ID"},
			{Key: "user", Label: "User"},
			{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: statusSeverities},
			{Key: "exitstatus", Label: "Exit status", Type: plugin.ColumnBadge, Severities: statusSeverities},
			{Key: "starttime", Label: "Started", Type: plugin.ColumnDateTime},
			{Key: "endtime", Label: "Ended", Type: plugin.ColumnDateTime},
		}}},
		RawToggle: true,
	}
}

func diskFields() []plugin.ObjectDetailField {
	fields := []plugin.ObjectDetailField{}
	for _, prefix := range []string{"scsi", "virtio", "sata", "ide", "efidisk", "tpmstate", "unused"} {
		limit := 4
		if prefix == "scsi" {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			key := prefix + fmt.Sprint(i)
			fields = append(fields, plugin.ObjectDetailField{Key: key, Label: key})
		}
	}
	return fields
}

func netFields() []plugin.ObjectDetailField {
	fields := []plugin.ObjectDetailField{}
	for i := 0; i < 8; i++ {
		key := "net" + fmt.Sprint(i)
		fields = append(fields, plugin.ObjectDetailField{Key: key, Label: key})
	}
	return fields
}

func mountFields() []plugin.ObjectDetailField {
	fields := []plugin.ObjectDetailField{}
	for i := 0; i < 8; i++ {
		key := "mp" + fmt.Sprint(i)
		fields = append(fields, plugin.ObjectDetailField{Key: key, Label: key})
	}
	return fields
}

// cpuMemMetrics declares compact CPU/Memory usage rows plus history lines.
func cpuMemMetrics() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Usage: []plugin.MetricUsage{
			{Key: "cpu", Label: "CPU usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "cpu", TotalKey: "cpuTotal", TotalType: plugin.ColumnNumber, TotalLabel: "of", Unit: "CPU(s)", WarnAt: 75, CriticalAt: 90}},
			{Key: "mem", Label: "Memory usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "mem", UsedKey: "memUsed", TotalKey: "memTotal", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 95}},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU", Unit: "%"},
			{Key: "mem", Label: "Memory", Unit: "%"},
		},
	}
}

func actions() []plugin.Action {
	acts := append(lifecycleActions("qemu"), lifecycleActions("lxc")...)
	acts = append(acts,
		plugin.Action{
			ID: "act.node.power", Label: "Power", Icon: icon("power"), RouteID: "proxmox.node.power",
			Params:  map[string]string{"node": "${resource.uid}"},
			Confirm: true, ConfirmText: "Reboot or shut down this node? Running guests are affected.",
		},
		plugin.Action{
			ID: "act.task.stop", Label: "Stop", Icon: icon("square"), RouteID: "proxmox.task.stop",
			Params:      map[string]string{"node": "${resource.namespace}", "upid": "${resource.uid}"},
			Confirm:     true,
			ConfirmText: "Stop this running Proxmox task?",
			EnabledWhen: whenStatus("running"),
			Bulk:        true,
		},
	)
	return append(acts, plugin.Action{
		ID: "act.backup.delete", Label: "Delete backup", Icon: icon("trash"), RouteID: "proxmox.backup.delete",
		Params:  map[string]string{"node": "${resource.namespace}", "storage": "${resource.name}", "volume": "${resource.uid}"},
		Confirm: true, ConfirmText: "Delete this backup archive? This cannot be undone.",
		VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "content", Op: plugin.OpEq, Value: "backup"}}},
		Bulk:        true,
	})
}

// whenStatus gates a guest action on the row's "status" field (running/stopped).
func whenStatus(statuses ...string) *plugin.Condition {
	return &plugin.Condition{AllOf: []plugin.Rule{{Field: "status", Op: plugin.OpIn, Value: statuses}}}
}

func lifecycleActions(kind string) []plugin.Action {
	gp := guestParams()
	acts := []plugin.Action{
		{ID: "act." + kind + ".start", Label: "Start", Icon: icon("play"), RouteID: "proxmox." + kind + ".start", Params: gp, EnabledWhen: whenStatus("stopped"), VisibleWhen: instanceOnly(), Group: "Power"},
		{ID: "act." + kind + ".shutdown", Label: "Shutdown", Icon: icon("power"), RouteID: "proxmox." + kind + ".shutdown", Params: gp, Confirm: true, ConfirmText: "Gracefully shut down this guest?", EnabledWhen: whenStatus("running"), VisibleWhen: instanceOnly(), Group: "Power"},
		{ID: "act." + kind + ".reboot", Label: "Reboot", Icon: icon("rotate-cw"), RouteID: "proxmox." + kind + ".reboot", Params: gp, Confirm: true, ConfirmText: "Reboot this guest?", EnabledWhen: whenStatus("running"), VisibleWhen: instanceOnly(), Group: "Power"},
		{ID: "act." + kind + ".stop", Label: "Stop", Icon: icon("square"), RouteID: "proxmox." + kind + ".stop", Params: gp, Confirm: true, ConfirmText: "Force stop this guest? Unsaved state is lost.", EnabledWhen: whenStatus("running"), VisibleWhen: instanceOnly(), Group: "Power"},
		{ID: "act." + kind + ".migrate", Label: "Migrate", Icon: icon("route"), RouteID: "proxmox." + kind + ".migrate", Params: gp, VisibleWhen: instanceOnly(), Group: "Manage"},
		{ID: "act." + kind + ".snapshot.create", Label: "Snapshot", Icon: icon("camera"), RouteID: "proxmox." + kind + ".snapshot.create", Params: gp, OnSuccess: &plugin.ActionSuccess{SelectTab: "snapshots"}, VisibleWhen: instanceOnly(), Group: "Snapshots"},
		{ID: "act." + kind + ".backup", Label: "Backup now", Icon: icon("archive"), RouteID: "proxmox." + kind + ".backup", Params: gp, OnSuccess: &plugin.ActionSuccess{SelectTab: "backups"}, VisibleWhen: instanceOnly(), Group: "Snapshots"},
		{ID: "act." + kind + ".clone", Label: "Clone", Icon: icon("copy"), RouteID: "proxmox." + kind + ".clone", Params: gp, Group: "Manage"},
		{ID: "act." + kind + ".destroy", Label: "Destroy", Icon: icon("trash-2"), RouteID: "proxmox." + kind + ".destroy", Params: gp, Confirm: true, ConfirmText: "Destroy this guest and all its disks? This cannot be undone.", EnabledWhen: whenStatus("stopped")},
	}
	if kind == "qemu" {
		acts = append(acts,
			plugin.Action{ID: "act.qemu.suspend", Label: "Suspend", Icon: icon("power-off"), RouteID: "proxmox.qemu.suspend", Params: gp, Confirm: true, ConfirmText: "Suspend this VM to disk?", EnabledWhen: whenStatus("running"), VisibleWhen: instanceOnly(), Group: "Power"},
			plugin.Action{ID: "act.qemu.resume", Label: "Resume", Icon: icon("play"), RouteID: "proxmox.qemu.resume", Params: gp, VisibleWhen: instanceOnly(), Group: "Power"},
			plugin.Action{ID: "act.qemu.resize", Label: "Resize disk", Icon: icon("scaling"), RouteID: "proxmox.qemu.resize", Params: gp, VisibleWhen: instanceOnly(), Group: "Manage"},
		)
	}
	snapParams := map[string]string{"node": "${resource.namespace}", "vmid": "${resource.name}", "snapname": "${resource.uid}"}
	return append(acts,
		plugin.Action{ID: "act." + kind + ".snapshot.rollback", Label: "Rollback", Icon: icon("rotate-cw"), RouteID: "proxmox." + kind + ".snapshot.rollback", Params: snapParams, Confirm: true, ConfirmText: "Roll back to this snapshot? Current state is lost.", VisibleWhen: instanceOnly()},
		plugin.Action{ID: "act." + kind + ".snapshot.delete", Label: "Delete", Icon: icon("trash"), RouteID: "proxmox." + kind + ".snapshot.delete", Params: snapParams, Confirm: true, ConfirmText: "Delete this snapshot?", VisibleWhen: instanceOnly(), Bulk: true},
		plugin.Action{ID: "act." + kind + ".backup.restore", Label: "Restore", Icon: icon("upload"), RouteID: "proxmox." + kind + ".restore", Params: map[string]string{"node": "${resource.namespace}", "archive": "${resource.uid}"}, Confirm: true, ConfirmText: "Restore this backup archive. Existing guests can be overwritten only when enabled in the form.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "content", Op: plugin.OpEq, Value: "backup"}, {Field: "guestType", Op: plugin.OpEq, Value: kind}}}},
	)
}
