// Package servermonitor implements a host monitoring plugin.
package servermonitor

import (
	"context"
	"time"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.CurrentAPIVersion,
		Name:        "server_monitor",
		Version:     "0.1.0",
		Title:       "Server Monitor",
		Description: "Cross-platform host monitor for CPU, memory, disks, network interfaces, processes, services, and live utilization.",
		Icon:        plugin.Icon{Type: plugin.IconLucide, Value: "activity"},
		Category:    plugin.CategoryObservability,
		Config:      configSchema(),
		Capabilities: []plugin.Capability{
			"host", "metrics", "processes", "services", "disks", "disk_io", "network", "connections", "sessions", "sensors", "cpu",
		},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent},
		Agent: &plugin.AgentProfile{
			Proxy: plugin.ProxyTarget{Mode: plugin.AgentHostMonitor, Risk: plugin.RiskSafe},
			Install: []plugin.InstallArtifact{{
				Label:      "Container",
				Kind:       "docker-run",
				ConnectURL: plugin.ArtifactConnectURL{LocalhostHost: "host.docker.internal"},
				Template: "docker run --rm --name " + app.AgentBinary + " " +
					"{{if .LocalhostHostRequired}}--add-host={{.LocalhostHost}}:host-gateway {{end}}" +
					"--pid=host " +
					"-e HOST_PROC=/host/proc -e HOST_SYS=/host/sys -e HOST_ETC=/host/etc " +
					"-e SHELLCN_CONNECT_URL={{shellquote .ConnectURL}} " +
					"{{if .Insecure}}-e SHELLCN_INSECURE=1 {{end}}" +
					"-e SHELLCN_ENROLL_TOKEN={{shellquote .Token}} " +
					"-v /proc:/host/proc:ro -v /sys:/host/sys:ro -v /etc:/host/etc:ro " +
					"{{shellquote .Image}}",
			}, {
				Label: "Native binary",
				Kind:  "shell",
				Template: "./" + app.AgentBinary + " " +
					"-connect {{shellquote .ConnectURL}} " +
					"{{if .Insecure}}-insecure {{end}}" +
					"-token {{shellquote .Token}}",
			}, {
				Label: "PowerShell",
				Kind:  "powershell",
				Template: ".\\" + app.AgentBinary + ".exe " +
					"-connect {{shellquote .ConnectURL}} " +
					"{{if .Insecure}}-insecure {{end}}" +
					"-token {{shellquote .Token}}",
			}},
		},
		Layout:  plugin.LayoutTabs,
		Tabs:    tabs(),
		Streams: []plugin.Stream{{ID: "server_monitor.metrics", Kind: plugin.StreamMetrics, RouteID: "server_monitor.metrics"}},
	}
}

func (p *Plugin) Routes() []plugin.Route { return Routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return Connect(ctx, cfg)
}

func configSchema() plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{{
		Name: "Collection",
		Fields: []plugin.Field{
			{
				Key: "metrics_interval_seconds", Label: "Metrics interval", Type: plugin.FieldNumber,
				Default: 5, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 60}},
			},
			{
				Key: "process_limit", Label: "Process limit", Type: plugin.FieldNumber,
				Default: 1000, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 50}, {Type: plugin.ValidatorMax, Value: 5000}},
			},
			{
				Key: "connection_limit", Label: "Connection limit", Type: plugin.FieldNumber,
				Default: 1000, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 50}, {Type: plugin.ValidatorMax, Value: 10000}},
			},
		},
	}}}
}

func tabs() []plugin.Tab {
	return []plugin.Tab{
		{
			Key: "overview", Label: "Overview", Icon: lucide("layout-dashboard"), Panel: plugin.PanelDashboard,
			Config: plugin.DashboardConfig{Cells: []plugin.DashboardCell{
				{
					Key: "metrics", Label: "Live metrics", Panel: plugin.PanelMetrics, Span: 2,
					Source: &plugin.DataSource{RouteID: "server_monitor.metrics", Method: plugin.MethodWS},
					Config: summaryConfig(),
				},
				{
					Key: "cpumem", Label: "CPU & Memory", Panel: plugin.PanelMetrics, Span: 1,
					Source: &plugin.DataSource{RouteID: "server_monitor.metrics", Method: plugin.MethodWS},
					Config: cpuMemConfig(),
				},
				{
					Key: "throughput", Label: "Throughput", Panel: plugin.PanelMetrics, Span: 1,
					Source: &plugin.DataSource{RouteID: "server_monitor.metrics", Method: plugin.MethodWS},
					Config: throughputConfig(),
				},
				{
					Key: "system", Label: "System", Panel: plugin.PanelDocument, Span: 1,
					Source: &plugin.DataSource{RouteID: "server_monitor.overview"},
				},
				{
					Key: "disks", Label: "Disks", Panel: plugin.PanelTable, Span: 1,
					Source: &plugin.DataSource{RouteID: "server_monitor.disks"},
					Config: liveTableConfig(diskColumns(), 10000, sortBy("usedPct")),
				},
			}},
		},
		{Key: "processes", Label: "Processes", Icon: lucide("list-tree"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.processes"}, Config: liveTableConfig(processColumns(), 3000, sortBy("cpuPct"))},
		{Key: "services", Label: "Services", Icon: lucide("settings"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.services"}, Config: liveTableConfig(serviceColumns(), 10000, nil)},
		{Key: "disks", Label: "Disks", Icon: lucide("hard-drive"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.disks"}, Config: liveTableConfig(diskColumns(), 10000, sortBy("usedPct"))},
		{Key: "io", Label: "Disk IO", Icon: lucide("activity"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.disk_io"}, Config: liveTableConfig(diskIOColumns(), 3000, sortBy("writeBytes"))},
		{Key: "network", Label: "Network", Icon: lucide("network"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.network"}, Config: liveTableConfig(networkColumns(), 3000, sortBy("bytesRecv"))},
		{Key: "connections", Label: "Connections", Icon: lucide("radio-tower"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.connections"}, Config: liveTableConfig(connectionColumns(), 5000, nil)},
		{Key: "sessions", Label: "Sessions", Icon: lucide("users"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.users"}, Config: liveTableConfig(userColumns(), 15000, nil)},
		{Key: "sensors", Label: "Sensors", Icon: lucide("thermometer"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.sensors"}, Config: liveTableConfig(sensorColumns(), 5000, sortBy("temperature"))},
		{Key: "cpu", Label: "CPU", Icon: lucide("cpu"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "server_monitor.cpu"}, Config: tableConfig(cpuColumns())},
		{Key: "system", Label: "System", Icon: lucide("server"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "server_monitor.overview"}},
	}
}

func lucide(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

// summaryConfig is the full-width header card: the CPU/Mem/Swap gauges plus the
// process/load stats. The line charts live in their own cells (below) so they
// can sit in a grid.
func summaryConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Gauges: []plugin.MetricGauge{
			{Key: "cpuPct", Label: "CPU", Unit: "%", Max: 100},
			{Key: "memPct", Label: "Memory", Unit: "%", Max: 100},
			{Key: "swapPct", Label: "Swap", Unit: "%", Max: 100},
		},
		Stats: []plugin.MetricStat{
			{Key: "processes", Label: "Processes"},
			{Key: "load1", Label: "Load 1m"},
			{Key: "load5", Label: "Load 5m"},
		},
	}
}

// cpuMemConfig charts CPU and Memory over time on one shared 0–100 axis.
func cpuMemConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Series: []plugin.MetricSeries{
			{Key: "cpuPct", Label: "CPU", Unit: "%"},
			{Key: "memPct", Label: "Memory", Unit: "%"},
		},
		History: 120,
	}
}

// throughputConfig charts network and disk I/O as per-second rates (derived in
// the metrics stream), kept off the percentage chart so their byte scale doesn't
// flatten it.
func throughputConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Series: []plugin.MetricSeries{
			{Key: "netRecvRate", Label: "Net in", Unit: "bytes"},
			{Key: "netSentRate", Label: "Net out", Unit: "bytes"},
			{Key: "diskReadRate", Label: "Disk read", Unit: "bytes"},
			{Key: "diskWriteRate", Label: "Disk write", Unit: "bytes"},
		},
		History: 120,
	}
}

func tableConfig(columns []plugin.Column) plugin.TableConfig {
	return plugin.TableConfig{Columns: columns, Exportable: true, RowClick: plugin.RowClickDetail}
}

func liveTableConfig(columns []plugin.Column, intervalMs int, sort *plugin.SortKey) plugin.TableConfig {
	return plugin.TableConfig{
		Columns:           columns,
		RefreshIntervalMs: intervalMs,
		DefaultSort:       sort,
		Exportable:        true,
		RowClick:          plugin.RowClickDetail,
	}
}

func sortBy(field string) *plugin.SortKey { return &plugin.SortKey{Field: field, Desc: true} }

func prec(n int) *int { return &n }

func processColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "pid", Label: "PID", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "user", Label: "User", Sortable: true},
		{Key: "cpuPct", Label: "CPU", Type: plugin.ColumnPercent, Precision: prec(1), Sortable: true},
		{Key: "memPct", Label: "Mem", Type: plugin.ColumnPercent, Precision: prec(1), Sortable: true},
		{Key: "rss", Label: "RSS", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "threads", Label: "Threads", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "createdAt", Label: "Started", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "cmdline", Label: "Command"},
	}
}

func serviceColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "unit", Label: "Unit", Sortable: true},
		{Key: "active", Label: "Active", Type: plugin.ColumnBadge, Sortable: true, Severities: map[string]plugin.Severity{
			"running": plugin.SeveritySuccess, "active": plugin.SeveritySuccess,
			"stopped": plugin.SeveritySecondary, "inactive": plugin.SeveritySecondary,
			"failed": plugin.SeverityDanger,
		}},
		{Key: "sub", Label: "State", Sortable: true},
		{Key: "load", Label: "Load", Sortable: true},
		{Key: "description", Label: "Description"},
	}
}

func diskColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "device", Label: "Device", Sortable: true},
		{Key: "mountpoint", Label: "Mount", Sortable: true},
		{Key: "fstype", Label: "FS", Sortable: true},
		{Key: "usedPct", Label: "Used", Type: plugin.ColumnPercent, Precision: prec(1), Sortable: true},
		{Key: "total", Label: "Total", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "used", Label: "Used", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "free", Label: "Free", Type: plugin.ColumnBytes, Sortable: true},
	}
}

func diskIOColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Device", Sortable: true},
		{Key: "readBytes", Label: "Read", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "writeBytes", Label: "Written", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "readCount", Label: "Reads", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "writeCount", Label: "Writes", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "readTime", Label: "Read time", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "writeTime", Label: "Write time", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "ioTime", Label: "IO time", Type: plugin.ColumnNumber, Sortable: true},
	}
}

func networkColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "addresses", Label: "Addresses"},
		{Key: "mtu", Label: "MTU", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "bytesRecv", Label: "In", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "bytesSent", Label: "Out", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "packetsRecv", Label: "Packets in", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "packetsSent", Label: "Packets out", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "flags", Label: "Flags"},
	}
}

func connectionColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "pid", Label: "PID", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "localAddr", Label: "Local", Sortable: true},
		{Key: "remoteAddr", Label: "Remote", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true, Severities: map[string]plugin.Severity{
			"established": plugin.SeveritySuccess,
			"listen":      plugin.SeverityInfo,
			"time_wait":   plugin.SeveritySecondary,
			"close_wait":  plugin.SeverityWarn,
		}},
		{Key: "family", Label: "Family", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "type", Label: "Type", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "uids", Label: "UIDs"},
	}
}

func userColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "user", Label: "User", Sortable: true},
		{Key: "terminal", Label: "Terminal", Sortable: true},
		{Key: "host", Label: "Host", Sortable: true},
		{Key: "startedAt", Label: "Started", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func sensorColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "sensor", Label: "Sensor", Sortable: true},
		{Key: "temperature", Label: "Temp °C", Type: plugin.ColumnNumber, Precision: prec(1), Sortable: true},
		{Key: "high", Label: "High", Type: plugin.ColumnNumber, Precision: prec(1), Sortable: true},
		{Key: "critical", Label: "Critical", Type: plugin.ColumnNumber, Precision: prec(1), Sortable: true},
	}
}

func cpuColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "cpu", Label: "CPU", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "modelName", Label: "Model", Sortable: true},
		{Key: "vendor", Label: "Vendor", Sortable: true},
		{Key: "mhz", Label: "MHz", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "cores", Label: "Cores", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "cacheSize", Label: "Cache", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "flags", Label: "Flags"},
	}
}

func intervalFromConfig(cfg plugin.ConnectConfig) time.Duration {
	seconds, ok := cfg.Int("metrics_interval_seconds")
	if !ok || seconds <= 0 {
		seconds = 5
	}
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}
