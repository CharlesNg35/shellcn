// Package redis implements the Redis protocol plugin.
package redis

import (
	"context"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

const redisIconSvg = `<svg width="800px" height="800px" viewBox="0 -18 256 256" xmlns="http://www.w3.org/2000/svg" preserveAspectRatio="xMinYMin meet"><path d="M245.97 168.943c-13.662 7.121-84.434 36.22-99.501 44.075-15.067 7.856-23.437 7.78-35.34 2.09-11.902-5.69-87.216-36.112-100.783-42.597C3.566 169.271 0 166.535 0 163.951v-25.876s98.05-21.345 113.879-27.024c15.828-5.679 21.32-5.884 34.79-.95 13.472 4.936 94.018 19.468 107.331 24.344l-.006 25.51c.002 2.558-3.07 5.364-10.024 8.988" fill="#912626"/><path d="M245.965 143.22c-13.661 7.118-84.431 36.218-99.498 44.072-15.066 7.857-23.436 7.78-35.338 2.09-11.903-5.686-87.214-36.113-100.78-42.594-13.566-6.485-13.85-10.948-.524-16.166 13.326-5.22 88.224-34.605 104.055-40.284 15.828-5.677 21.319-5.884 34.789-.948 13.471 4.934 83.819 32.935 97.13 37.81 13.316 4.881 13.827 8.9.166 16.02" fill="#C6302B"/><path d="M245.97 127.074c-13.662 7.122-84.434 36.22-99.501 44.078-15.067 7.853-23.437 7.777-35.34 2.087-11.903-5.687-87.216-36.112-100.783-42.597C3.566 127.402 0 124.67 0 122.085V96.206s98.05-21.344 113.879-27.023c15.828-5.679 21.32-5.885 34.79-.95C162.142 73.168 242.688 87.697 256 92.574l-.006 25.513c.002 2.557-3.07 5.363-10.024 8.987" fill="#912626"/><path d="M245.965 101.351c-13.661 7.12-84.431 36.218-99.498 44.075-15.066 7.854-23.436 7.777-35.338 2.087-11.903-5.686-87.214-36.112-100.78-42.594-13.566-6.483-13.85-10.947-.524-16.167C23.151 83.535 98.05 54.148 113.88 48.47c15.828-5.678 21.319-5.884 34.789-.949 13.471 4.934 83.819 32.933 97.13 37.81 13.316 4.88 13.827 8.9.166 16.02" fill="#C6302B"/><path d="M245.97 83.653c-13.662 7.12-84.434 36.22-99.501 44.078-15.067 7.854-23.437 7.777-35.34 2.087-11.903-5.687-87.216-36.113-100.783-42.595C3.566 83.98 0 81.247 0 78.665v-25.88s98.05-21.343 113.879-27.021c15.828-5.68 21.32-5.884 34.79-.95C162.142 29.749 242.688 44.278 256 49.155l-.006 25.512c.002 2.555-3.07 5.361-10.024 8.986" fill="#912626"/><path d="M245.965 57.93c-13.661 7.12-84.431 36.22-99.498 44.074-15.066 7.854-23.436 7.777-35.338 2.09C99.227 98.404 23.915 67.98 10.35 61.497-3.217 55.015-3.5 50.55 9.825 45.331 23.151 40.113 98.05 10.73 113.88 5.05c15.828-5.679 21.319-5.883 34.789-.948 13.471 4.935 83.819 32.934 97.13 37.811 13.316 4.876 13.827 8.897.166 16.017" fill="#C6302B"/><path d="M159.283 32.757l-22.01 2.285-4.927 11.856-7.958-13.23-25.415-2.284 18.964-6.839-5.69-10.498 17.755 6.944 16.738-5.48-4.524 10.855 17.067 6.391M131.032 90.275L89.955 73.238l58.86-9.035-17.783 26.072M74.082 39.347c17.375 0 31.46 5.46 31.46 12.194 0 6.736-14.085 12.195-31.46 12.195s-31.46-5.46-31.46-12.195c0-6.734 14.085-12.194 31.46-12.194" fill="#FFF"/><path d="M185.295 35.998l34.836 13.766-34.806 13.753-.03-27.52" fill="#621B1C"/><path d="M146.755 51.243l38.54-15.245.03 27.519-3.779 1.478-34.791-13.752" fill="#9A2928"/></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "Redis",
		Description:         "Redis key browser with typed string, hash, list, set, sorted set, metadata, clients, and pub/sub views.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: redisIconSvg},
		Category:            plugin.CategoryDatabases,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"kv", "keys", "pubsub", "terminal"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Scope:               []plugin.ScopeFilter{databaseScope()},
		Tabs: []plugin.Panel{
			{Key: "overview", Label: "Overview", Icon: icon("gauge"), Type: plugin.PanelDashboard, Config: overviewDashboard()},
			{Key: "keys", Label: "Keys", Icon: icon("key-round"), Type: plugin.PanelKV, Source: &plugin.DataSource{RouteID: "redis.keys.list"}, Config: plugin.KVConfig{
				CreateRouteID: "redis.key.write", ReadRouteID: "redis.key.read", WriteRouteID: "redis.key.write", DeleteRouteID: "redis.key.delete", KeyParam: "key", Writable: true,
				ValueTypes: []string{"string", "hash", "list", "set", "zset"},
			}},
			{Key: "console", Label: "Console", Icon: icon("terminal"), Type: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "redis.terminal", Method: plugin.MethodWS}, Config: plugin.TerminalConfig{Zoom: true, Search: true}},
			{Key: "clients", Label: "Clients", Icon: icon("users"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "redis.clients.list"}, Config: clientsTableConfig()},
			{Key: "channels", Label: "Channels", Icon: icon("radio-tower"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "redis.channels.list"}, Config: channelsTableConfig()},
			{Key: "info", Label: "Info", Icon: icon("file-text"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "redis.info"}, Config: infoDetailConfig()},
		},
		Streams: []plugin.Stream{
			{ID: "redis.terminal", Kind: plugin.StreamTerminal, RouteID: "redis.terminal"},
		},
	}
}

func (p *Plugin) Routes() []plugin.Route { return routes() }

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return connect(ctx, cfg)
}

func icon(name string) plugin.Icon {
	return plugin.Icon{Type: plugin.IconLucide, Value: name}
}

func infoDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Server", Fields: []plugin.ObjectDetailField{
				{Key: "redis_version", Label: "Version", Copy: true},
				{Key: "redis_mode", Label: "Mode", Type: plugin.ColumnBadge},
				{Key: "role", Label: "Role", Type: plugin.ColumnBadge},
			}},
			{Title: "Clients", Fields: []plugin.ObjectDetailField{
				{Key: "connected_clients", Label: "Connected", Type: plugin.ColumnNumber},
				{Key: "blocked_clients", Label: "Blocked", Type: plugin.ColumnNumber},
				{Key: "tracking_clients", Label: "Tracking", Type: plugin.ColumnNumber},
			}},
			{Title: "Stats", Fields: []plugin.ObjectDetailField{
				{Key: "total_commands_processed", Label: "Commands", Type: plugin.ColumnNumber},
				{Key: "instantaneous_ops_per_sec", Label: "Ops/sec", Type: plugin.ColumnNumber},
				{Key: "keyspace_hits", Label: "Keyspace hits", Type: plugin.ColumnNumber},
				{Key: "keyspace_misses", Label: "Keyspace misses", Type: plugin.ColumnNumber},
			}},
		},
	}
}

func databaseScope() plugin.ScopeFilter {
	return plugin.ScopeFilter{
		Param:         databaseScopeParam,
		Label:         "Database",
		Icon:          icon("database"),
		Control:       plugin.ScopeSelect,
		DisableSearch: true,
		OptionsSource: &plugin.DataSource{RouteID: "redis.databases.list"},
		ValueField:    "value",
		LabelField:    "label",
		DefaultValue:  "0",
	}
}

func overviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "Connection", Fields: []plugin.ObjectDetailField{
				{Key: "address", Label: "Address", Copy: true},
				{Key: "database", Label: "Database", Type: plugin.ColumnNumber},
				{Key: "readOnly", Label: "Read only", Type: plugin.ColumnBool},
			}},
			{Title: "Activity", Fields: []plugin.ObjectDetailField{
				{Key: "role", Label: "Role", Type: plugin.ColumnBadge},
				{Key: "connected_clients", Label: "Clients", Type: plugin.ColumnNumber},
				{Key: "instantaneous_ops_per_sec", Label: "Ops/sec", Type: plugin.ColumnNumber},
			}},
		},
	}
}

// overviewDashboard keeps the first page compact. Full server INFO, clients,
// and channels are available in their dedicated tabs.
func overviewDashboard() plugin.DashboardConfig {
	return plugin.DashboardConfig{Cells: []plugin.Panel{
		{Key: "server", Label: "Server summary", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "redis.overview"}, Config: overviewDetailConfig(), Span: 2},
	}}
}

func clientsTableConfig() plugin.TableConfig {
	return plugin.TableConfig{
		Columns:           clientColumns(),
		EmptyText:         "No connected clients.",
		Exportable:        true,
		RefreshIntervalMs: 5000,
		RowClick:          plugin.RowClickDetail,
		DefaultSort:       &plugin.SortKey{Field: "id"},
	}
}

func channelsTableConfig() plugin.TableConfig {
	return plugin.TableConfig{
		Columns:           channelColumns(),
		EmptyText:         "No active pub/sub channels.",
		Exportable:        true,
		RefreshIntervalMs: 5000,
		RowClick:          plugin.RowClickDetail,
		DefaultSort:       &plugin.SortKey{Field: "name"},
	}
}

func clientColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "id", Label: "ID", Sortable: true},
		{Key: "addr", Label: "Address", Sortable: true},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "db", Label: "DB", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "user", Label: "User"},
		{Key: "cmd", Label: "Command"},
		{Key: "flags", Label: "Flags"},
		{Key: "sub", Label: "Subs", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "psub", Label: "Patterns", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "omem", Label: "Output memory", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "age", Label: "Age", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "idle", Label: "Idle", Type: plugin.ColumnNumber, Sortable: true},
	}
}

func channelColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Channel", Sortable: true},
		{Key: "subscribers", Label: "Subscribers", Type: plugin.ColumnNumber, Sortable: true},
	}
}
