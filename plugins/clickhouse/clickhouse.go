// Package clickhouse implements the ClickHouse protocol plugin.
package clickhouse

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

const clickHouseIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 150 150"><rect y="0" width="150" height="150" rx="18" fill="#ffcc01"/><path fill="#161616" d="M30,28.3c0-.6.5-1.1,1.1-1.1h8.4c.6,0,1.1.5,1.1,1.1v93.3c0,.6-.5,1.1-1.1,1.1h-8.4c-.6,0-1.1-.5-1.1-1.1V28.3Z"/><path fill="#161616" d="M51.2,28.3c0-.6.5-1.1,1.1-1.1h8.4c.6,0,1.1.5,1.1,1.1v93.3c0,.6-.5,1.1-1.1,1.1h-8.4c-.6,0-1.1-.5-1.1-1.1V28.3Z"/><path fill="#161616" d="M72.4,28.3c0-.6.5-1.1,1.1-1.1h8.4c.6,0,1.1.5,1.1,1.1v93.3c0,.6-.5,1.1-1.1,1.1h-8.4c-.6,0-1.1-.5-1.1-1.1V28.3Z"/><path fill="#161616" d="M93.7,28.3c0-.6.5-1.1,1.1-1.1h8.4c.6,0,1.1.5,1.1,1.1v93.3c0,.6-.5,1.1-1.1,1.1h-8.4c-.6,0-1.1-.5-1.1-1.1V28.3Z"/><path fill="#161616" d="M114.9,65.5c0-.6.5-1.1,1.1-1.1h8.4c.6,0,1.1.5,1.1,1.1v19c0,.6-.5,1.1-1.1,1.1h-8.4c-.6,0-1.1-.5-1.1-1.1v-19Z"/></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "ClickHouse",
		Description:         "ClickHouse cockpit with databases, tables, views, dictionaries, mutations, merges, processes, users, SQL editor, DDL helpers, and safety controls.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: clickHouseIconSVG},
		Category:            plugin.CategoryDatabases,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"sql", "schema", "tables", "query_editor", "analytics", "cluster"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSidebarTree,
		Tree:                tree(),
		Resources:           resources(),
		Actions:             actions(),
		Streams: []plugin.Stream{
			{ID: "clickhouse.query", Kind: plugin.StreamLogs, RouteID: "clickhouse.query"},
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

func tree() []plugin.TreeGroup {
	return []plugin.TreeGroup{
		{Key: "databases", Label: "Databases", Icon: icon("database"), Source: plugin.DataSource{RouteID: "clickhouse.databases.tree"}, Ref: &plugin.ResourceRef{Kind: "server", Name: "Databases", UID: "server"}},
		{Key: "dictionaries", Label: "Dictionaries", Icon: icon("book-open"), Source: plugin.DataSource{RouteID: "clickhouse.dictionaries.tree"}, ResourceKind: "dictionary"},
		{Key: "mutations", Label: "Mutations", Icon: icon("git-compare-arrows"), ResourceKind: "mutation"},
		{Key: "merges", Label: "Merges", Icon: icon("merge"), ResourceKind: "merge"},
		{Key: "processes", Label: "Processes", Icon: icon("activity"), ResourceKind: "process"},
		{Key: "users", Label: "Users", Icon: icon("users"), Source: plugin.DataSource{RouteID: "clickhouse.users.tree"}, ResourceKind: "user"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		serverResource(),
		databaseResource(),
		tableResource(),
		viewResource(),
		dictionaryResource(),
		mutationResource(),
		mergeResource(),
		processResource(),
		userResource(),
	}
}

// serverResource is the connection-level view opened by clicking the Databases
// tree group: the database list plus a SQL console.
func serverResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "server", Title: "Databases",
		List: plugin.DataSource{RouteID: "clickhouse.databases.list"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Databases"},
			Tabs: []plugin.Tab{
				{Key: "databases", Label: "Databases", Icon: icon("database"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.databases.list"}, Config: plugin.TableConfig{ActionIDs: []string{"clickhouse.database.create"}}.Map()},
				{Key: "console", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "clickhouse.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT version();")},
			},
		},
	}
}

func databaseResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "database", Title: "Databases",
		List:          plugin.DataSource{RouteID: "clickhouse.databases.list"},
		ListActionIDs: []string{"clickhouse.database.create"},
		Columns: []plugin.Column{
			{Key: "name", Label: "Database", Sortable: true},
			{Key: "engine", Label: "Engine", Sortable: true},
			{Key: "tables", Label: "Tables", Type: plugin.ColumnNumber, Sortable: true},
			{Key: "views", Label: "Views", Type: plugin.ColumnNumber, Sortable: true},
			{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
			{Key: "comment", Label: "Comment"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.database.overview", Params: map[string]string{"database": "${resource.uid}"}}},
				{Key: "tables", Label: "Tables", Icon: icon("table-2"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.tables.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: tableColumns(), ActionIDs: []string{"clickhouse.table.create"}}.Map()},
				{Key: "views", Label: "Views", Icon: icon("panel-top"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.views.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: viewColumns(), RowActionIDs: []string{"clickhouse.view.drop"}}.Map()},
				{Key: "dictionaries", Label: "Dictionaries", Icon: icon("book-open"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.dictionaries.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: dictionaryColumns()}.Map()},
				{Key: "mutations", Label: "Mutations", Icon: icon("git-compare-arrows"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.mutations.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: mutationColumns()}.Map()},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "clickhouse.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT version();")},
			},
		},
	}
}

func tableResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "table", Title: "Tables",
		List:         plugin.DataSource{RouteID: "clickhouse.tables.list"},
		Columns:      tableColumns(),
		RowActionIDs: []string{"clickhouse.column.add", "clickhouse.table.truncate", "clickhouse.table.drop"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}", ActionIDs: []string{"clickhouse.table.truncate", "clickhouse.table.drop"}},
			Tabs: []plugin.Tab{
				{Key: "data", Label: "Data", Icon: icon("table-properties"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.table.rows", Params: tableParams()}, Config: plugin.TableConfig{Exportable: true}.Map()},
				{Key: "columns", Label: "Columns", Icon: icon("columns-3"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.table.columns", Params: tableParams()}, Config: plugin.TableConfig{Columns: columnColumns(), ActionIDs: []string{"clickhouse.column.add"}, RowActionIDs: []string{"clickhouse.column.drop"}}.Map()},
				{Key: "indexes", Label: "Indexes", Icon: icon("key-round"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.table.indexes", Params: tableParams()}, Config: plugin.TableConfig{Columns: indexColumns(), RowActionIDs: []string{"clickhouse.index.drop"}}.Map()},
				{Key: "constraints", Label: "Constraints", Icon: icon("shield-check"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.table.constraints", Params: tableParams()}, Config: plugin.TableConfig{Columns: constraintColumns()}.Map()},
				{Key: "mutations", Label: "Mutations", Icon: icon("git-compare-arrows"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.mutations.list", Params: tableParams()}, Config: plugin.TableConfig{Columns: mutationColumns()}.Map()},
				{Key: "definition", Label: "Definition", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.table.definition", Params: tableParams()}},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "clickhouse.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT * FROM `${resource.namespace}`.`${resource.name}` LIMIT 100;")},
			},
		},
	}
}

func viewResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "view", Title: "Views",
		List: plugin.DataSource{RouteID: "clickhouse.views.list"}, Columns: viewColumns(),
		RowActionIDs: []string{"clickhouse.view.drop"},
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}", ActionIDs: []string{"clickhouse.view.drop"}}, Tabs: []plugin.Tab{
			{Key: "data", Label: "Data", Icon: icon("table-properties"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "clickhouse.view.rows", Params: tableParams()}, Config: plugin.TableConfig{Exportable: true}.Map()},
			{Key: "definition", Label: "Definition", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.view.definition", Params: tableParams()}},
			{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "clickhouse.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT * FROM `${resource.namespace}`.`${resource.name}` LIMIT 100;")},
		}},
	}
}

func dictionaryResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "dictionary", Title: "Dictionaries",
		List:    plugin.DataSource{RouteID: "clickhouse.dictionaries.list"},
		Columns: dictionaryColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.dictionary.overview", Params: tableParams()}},
		}},
	}
}

func mutationResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "mutation", Title: "Mutations",
		List:    plugin.DataSource{RouteID: "clickhouse.mutations.list"},
		Columns: mutationColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.mutation.overview", Params: map[string]string{"id": "${resource.uid}"}}},
		}},
	}
}

func mergeResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "merge", Title: "Merges",
		List:    plugin.DataSource{RouteID: "clickhouse.merges.list"},
		Columns: mergeColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.merge.overview", Params: map[string]string{"id": "${resource.uid}"}}},
		}},
	}
}

func processResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "process", Title: "Processes",
		List:    plugin.DataSource{RouteID: "clickhouse.processes.list"},
		Columns: processColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.process.overview", Params: map[string]string{"id": "${resource.uid}"}}},
		}},
	}
}

func userResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "user", Title: "Users",
		List: plugin.DataSource{RouteID: "clickhouse.users.list"},
		Columns: []plugin.Column{
			{Key: "user", Label: "User", Sortable: true},
			{Key: "auth_type", Label: "Auth"},
			{Key: "storage", Label: "Storage"},
		},
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "clickhouse.user.overview", Params: map[string]string{"user": "${resource.name}"}}},
		}},
	}
}

func tableParams() map[string]string {
	return map[string]string{"database": "${resource.namespace}", "table": "${resource.name}"}
}

func queryConfig(initial string) map[string]any {
	return plugin.QueryEditorConfig{
		Language:          "sql",
		Label:             "SQL",
		ExecuteLabel:      "Run query",
		CancelLabel:       "Cancel query",
		RunningLabel:      "Running...",
		EmptyText:         "Run a query to see results.",
		InitialQuery:      initial,
		CancelRouteID:     "clickhouse.query.cancel",
		CompletionRouteID: "clickhouse.completion",
		Exportable:        true,
	}.Map()
}

func tableColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Table", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "engine", Label: "Engine"}, {Key: "rows", Label: "Rows", Type: plugin.ColumnNumber, Sortable: true}, {Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true}, {Key: "modified", Label: "Modified", Type: plugin.ColumnDateTime}, {Key: "comment", Label: "Comment"}}
}

func viewColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "View", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "engine", Label: "Engine"}, {Key: "modified", Label: "Modified", Type: plugin.ColumnDateTime}, {Key: "comment", Label: "Comment"}}
}

func dictionaryColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Dictionary", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "status", Label: "Status"}, {Key: "type", Label: "Type"}, {Key: "origin", Label: "Origin"}, {Key: "bytes_allocated", Label: "Bytes", Type: plugin.ColumnBytes}, {Key: "element_count", Label: "Elements", Type: plugin.ColumnNumber}}
}

func mutationColumns() []plugin.Column {
	return []plugin.Column{{Key: "mutation_id", Label: "Mutation", Sortable: true}, {Key: "database", Label: "Database"}, {Key: "table", Label: "Table"}, {Key: "command", Label: "Command"}, {Key: "create_time", Label: "Created", Type: plugin.ColumnDateTime}, {Key: "is_done", Label: "Done", Type: plugin.ColumnBool}, {Key: "latest_fail_reason", Label: "Last failure"}}
}

func mergeColumns() []plugin.Column {
	return []plugin.Column{{Key: "id", Label: "Merge"}, {Key: "database", Label: "Database"}, {Key: "table", Label: "Table"}, {Key: "elapsed", Label: "Elapsed", Type: plugin.ColumnNumber}, {Key: "progress", Label: "Progress", Type: plugin.ColumnNumber}, {Key: "num_parts", Label: "Parts", Type: plugin.ColumnNumber}}
}

func processColumns() []plugin.Column {
	return []plugin.Column{{Key: "query_id", Label: "Query", Sortable: true}, {Key: "user", Label: "User"}, {Key: "address", Label: "Address"}, {Key: "elapsed", Label: "Elapsed", Type: plugin.ColumnNumber}, {Key: "read_rows", Label: "Read rows", Type: plugin.ColumnNumber}, {Key: "memory_usage", Label: "Memory", Type: plugin.ColumnBytes}, {Key: "query", Label: "SQL"}}
}

func columnColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Column", Sortable: true}, {Key: "type", Label: "Type"}, {Key: "default_kind", Label: "Default kind"}, {Key: "default_expression", Label: "Default"}, {Key: "position", Label: "Position", Type: plugin.ColumnNumber, Sortable: true}, {Key: "comment", Label: "Comment"}}
}

func indexColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Index", Sortable: true}, {Key: "expression", Label: "Expression"}, {Key: "type", Label: "Type"}, {Key: "granularity", Label: "Granularity", Type: plugin.ColumnNumber}}
}

func constraintColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Constraint", Sortable: true}, {Key: "type", Label: "Type"}, {Key: "expression", Label: "Expression"}}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: "clickhouse.database.create", Label: "Create database", Icon: icon("plus"), RouteID: "clickhouse.database.create"},
		{ID: "clickhouse.table.create", Label: "Create table", Icon: icon("plus"), RouteID: "clickhouse.table.create", Params: map[string]string{"database": "${resource.uid}"}, OnSuccess: &plugin.ActionSuccess{SelectTab: "tables"}},
		{ID: "clickhouse.column.add", Label: "Add column", Icon: icon("columns-3"), RouteID: "clickhouse.column.add", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "clickhouse.column.drop", Label: "Drop column", Icon: icon("trash"), RouteID: "clickhouse.column.drop", Params: map[string]string{"database": "${resource.scope}", "table": "${resource.namespace}", "name": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this column? Its data is permanently removed.", OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "clickhouse.index.drop", Label: "Drop index", Icon: icon("trash"), RouteID: "clickhouse.index.drop", Params: map[string]string{"database": "${resource.scope}", "table": "${resource.namespace}", "name": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this data-skipping index?", OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: "clickhouse.table.truncate", Label: "Truncate", Icon: icon("trash"), RouteID: "clickhouse.table.truncate", Params: tableParams(), Confirm: true, ConfirmText: "Truncate this table? Every row will be deleted."},
		{ID: "clickhouse.table.drop", Label: "Drop", Icon: icon("trash-2"), RouteID: "clickhouse.table.drop", Params: tableParams(), Confirm: true, ConfirmText: "Drop this table? The table definition and data will be permanently deleted."},
		{ID: "clickhouse.view.drop", Label: "Drop", Icon: icon("trash-2"), RouteID: "clickhouse.view.drop", Params: map[string]string{"database": "${resource.namespace}", "view": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this view?"},
	}
}
