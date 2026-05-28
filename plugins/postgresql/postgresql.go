// Package postgresql implements the PostgreSQL protocol plugin.
package postgresql

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

const postgresIconSvg = `<?xml version="1.0"?><!doctypehtml> <svg height=445.383pt viewBox="0 0 432.071 445.383"width=432.071pt xml:space=preserve xmlns=http://www.w3.org/2000/svg><g id=original style=fill-rule:nonzero;clip-rule:nonzero;stroke:#000000;stroke-miterlimit:4></g><g id=Layer_x0020_3 style=fill-rule:nonzero;clip-rule:nonzero;fill:none;stroke:#FFFFFF;stroke-width:12.4651;stroke-linecap:round;stroke-linejoin:round;stroke-miterlimit:4><path d="M323.205,324.227c2.833-23.601,1.984-27.062,19.563-23.239l4.463,0.392c13.517,0.615,31.199-2.174,41.587-7c22.362-10.376,35.622-27.7,13.572-23.148c-50.297,10.376-53.755-6.655-53.755-6.655c53.111-78.803,75.313-178.836,56.149-203.322    C352.514-5.534,262.036,26.049,260.522,26.869l-0.482,0.089c-9.938-2.062-21.06-3.294-33.554-3.496c-22.761-0.374-40.032,5.967-53.133,15.904c0,0-161.408-66.498-153.899,83.628c1.597,31.936,45.777,241.655,98.47,178.31    c19.259-23.163,37.871-42.748,37.871-42.748c9.242,6.14,20.307,9.272,31.912,8.147l0.897-0.765c-0.281,2.876-0.157,5.689,0.359,9.019c-13.572,15.167-9.584,17.83-36.723,23.416c-27.457,5.659-11.326,15.734-0.797,18.367c12.768,3.193,42.305,7.716,62.268-20.224    l-0.795,3.188c5.325,4.26,4.965,30.619,5.72,49.452c0.756,18.834,2.017,36.409,5.856,46.771c3.839,10.36,8.369,37.05,44.036,29.406c29.809-6.388,52.6-15.582,54.677-101.107"style=fill:#000000;stroke:#000000;stroke-width:37.3953;stroke-linecap:butt;stroke-linejoin:miter /><path d="M402.395,271.23c-50.302,10.376-53.76-6.655-53.76-6.655c53.111-78.808,75.313-178.843,56.153-203.326c-52.27-66.785-142.752-35.2-144.262-34.38l-0.486,0.087c-9.938-2.063-21.06-3.292-33.56-3.496c-22.761-0.373-40.026,5.967-53.127,15.902    c0,0-161.411-66.495-153.904,83.63c1.597,31.938,45.776,241.657,98.471,178.312c19.26-23.163,37.869-42.748,37.869-42.748c9.243,6.14,20.308,9.272,31.908,8.147l0.901-0.765c-0.28,2.876-0.152,5.689,0.361,9.019c-13.575,15.167-9.586,17.83-36.723,23.416    c-27.459,5.659-11.328,15.734-0.796,18.367c12.768,3.193,42.307,7.716,62.266-20.224l-0.796,3.188c5.319,4.26,9.054,27.711,8.428,48.969c-0.626,21.259-1.044,35.854,3.147,47.254c4.191,11.4,8.368,37.05,44.042,29.406c29.809-6.388,45.256-22.942,47.405-50.555    c1.525-19.631,4.976-16.729,5.194-34.28l2.768-8.309c3.192-26.611,0.507-35.196,18.872-31.203l4.463,0.392c13.517,0.615,31.208-2.174,41.591-7c22.358-10.376,35.618-27.7,13.573-23.148z"style=fill:#336791;stroke:none /><path d=M215.866,286.484c-1.385,49.516,0.348,99.377,5.193,111.495c4.848,12.118,15.223,35.688,50.9,28.045c29.806-6.39,40.651-18.756,45.357-46.051c3.466-20.082,10.148-75.854,11.005-87.281 /><path d=M173.104,38.256c0,0-161.521-66.016-154.012,84.109c1.597,31.938,45.779,241.664,98.473,178.316c19.256-23.166,36.671-41.335,36.671-41.335 /><path d=M260.349,26.207c-5.591,1.753,89.848-34.889,144.087,34.417c19.159,24.484-3.043,124.519-56.153,203.329 /><path d="M348.282,263.953c0,0,3.461,17.036,53.764,6.653c22.04-4.552,8.776,12.774-13.577,23.155c-18.345,8.514-59.474,10.696-60.146-1.069c-1.729-30.355,21.647-21.133,19.96-28.739c-1.525-6.85-11.979-13.573-18.894-30.338    c-6.037-14.633-82.796-126.849,21.287-110.183c3.813-0.789-27.146-99.002-124.553-100.599c-97.385-1.597-94.19,119.762-94.19,119.762"style=stroke-linejoin:bevel /><path d=M188.604,274.334c-13.577,15.166-9.584,17.829-36.723,23.417c-27.459,5.66-11.326,15.733-0.797,18.365c12.768,3.195,42.307,7.718,62.266-20.229c6.078-8.509-0.036-22.086-8.385-25.547c-4.034-1.671-9.428-3.765-16.361,3.994z /><path d=M187.715,274.069c-1.368-8.917,2.93-19.528,7.536-31.942c6.922-18.626,22.893-37.255,10.117-96.339c-9.523-44.029-73.396-9.163-73.436-3.193c-0.039,5.968,2.889,30.26-1.067,58.548c-5.162,36.913,23.488,68.132,56.479,64.938 /><path d=M172.517,141.7c-0.288,2.039,3.733,7.48,8.976,8.207c5.234,0.73,9.714-3.522,9.998-5.559c0.284-2.039-3.732-4.285-8.977-5.015c-5.237-0.731-9.719,0.333-9.996,2.367z style=fill:#FFFFFF;stroke-width:4.155;stroke-linecap:butt;stroke-linejoin:miter /><path d=M331.941,137.543c0.284,2.039-3.732,7.48-8.976,8.207c-5.238,0.73-9.718-3.522-10.005-5.559c-0.277-2.039,3.74-4.285,8.979-5.015c5.239-0.73,9.718,0.333,10.002,2.368z style=fill:#FFFFFF;stroke-width:2.0775;stroke-linecap:butt;stroke-linejoin:miter /><path d=M350.676,123.432c0.863,15.994-3.445,26.888-3.988,43.914c-0.804,24.748,11.799,53.074-7.191,81.435 /><path d=M0,60.232 style=stroke-width:3 /></g></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.2.0",
		Title:               "PostgreSQL",
		Description:         "PostgreSQL cockpit: browse every database in the cluster, edit table data inline, manage schemas/tables, and run scoped SQL.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: postgresIconSvg},
		Category:            plugin.CategoryDatabases,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"sql", "schema", "tables", "data_grid", "query_editor"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSidebarTree,
		Tree:                tree(),
		Resources:           resources(),
		Actions:             actions(),
		Streams: []plugin.Stream{
			{ID: "postgresql.query", Kind: plugin.StreamLogs, RouteID: "postgresql.query"},
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
		{Key: "databases", Label: "Databases", Icon: icon("database"), Source: plugin.DataSource{RouteID: "postgresql.tree.databases"}, Ref: &plugin.ResourceRef{Kind: "server", Name: "Databases", UID: "server"}},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		serverResource(),
		databaseResource(),
		schemaResource(),
		tableResource(),
		viewResource(),
		functionResource(),
		sequenceResource(),
	}
}

// serverResource is the connection-level view opened by clicking the Databases
// tree group: the database list plus a SQL console, so a user can browse and run
// arbitrary SQL without first opening a specific database.
func serverResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "server", Title: "Databases",
		List: plugin.DataSource{RouteID: "postgresql.databases.list"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Databases"},
			Tabs: []plugin.Tab{
				{Key: "databases", Label: "Databases", Icon: icon("database"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.databases.list"}, Config: plugin.TableConfig{ActionIDs: []string{"postgresql.database.create"}}.Map()},
				{Key: "console", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "postgresql.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT now();", nil)},
			},
		},
	}
}

func databaseResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "database", Title: "Databases",
		List:          plugin.DataSource{RouteID: "postgresql.databases.list"},
		Columns:       databaseColumns(),
		ListActionIDs: []string{"postgresql.database.create"},
		RowActionIDs:  []string{"postgresql.database.drop"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{"postgresql.schema.create", "postgresql.database.drop"}},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.database.overview", Params: map[string]string{"database": "${resource.uid}"}}},
				{Key: "schemas", Label: "Schemas", Icon: icon("folder-tree"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.schemas.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: schemaColumns(), RowActionIDs: []string{"postgresql.schema.drop"}}.Map()},
				{Key: "tables", Label: "Tables", Icon: icon("table-2"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.tables.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: tableColumns(), RowActionIDs: []string{"postgresql.table.truncate", "postgresql.table.drop"}}.Map()},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "postgresql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.uid}"}}, Config: queryConfig("SELECT now();", map[string]string{"database": "${resource.uid}"})},
			},
		},
	}
}

func schemaResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "schema", Title: "Schemas",
		List:    plugin.DataSource{RouteID: "postgresql.schemas.list"},
		Columns: schemaColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.schema.overview", Params: schemaParams()}},
				{Key: "tables", Label: "Tables", Icon: icon("table-2"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.tables.list", Params: schemaParams()}, Config: plugin.TableConfig{Columns: tableColumns(), ActionIDs: []string{"postgresql.table.create"}, RowActionIDs: []string{"postgresql.table.truncate", "postgresql.table.drop"}}.Map()},
				{Key: "views", Label: "Views", Icon: icon("panel-top"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.views.list", Params: schemaParams()}, Config: plugin.TableConfig{Columns: viewColumns(), RowActionIDs: []string{"postgresql.view.drop"}}.Map()},
				{Key: "functions", Label: "Functions", Icon: icon("function-square"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.functions.list", Params: schemaParams()}, Config: plugin.TableConfig{Columns: functionColumns()}.Map()},
				{Key: "sequences", Label: "Sequences", Icon: icon("list-ordered"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.sequences.list", Params: schemaParams()}, Config: plugin.TableConfig{Columns: sequenceColumns()}.Map()},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "postgresql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.scope}"}}, Config: queryConfig("SELECT now();", map[string]string{"database": "${resource.scope}"})},
			},
		},
	}
}

func tableResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "table", Title: "Tables",
		List:         plugin.DataSource{RouteID: "postgresql.tables.list"},
		Columns:      tableColumns(),
		RowActionIDs: []string{"postgresql.column.add", "postgresql.table.truncate", "postgresql.table.drop"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}", ActionIDs: []string{"postgresql.table.truncate", "postgresql.table.drop"}},
			Tabs: []plugin.Tab{
				{Key: "data", Label: "Data", Icon: icon("table"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.table.rows", Params: tableParams()}, Config: dataGridConfig()},
				{Key: "columns", Label: "Columns", Icon: icon("columns-3"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.table.columns", Params: tableParams()}, Config: plugin.TableConfig{Columns: columnColumns(), ActionIDs: []string{"postgresql.column.add"}, RowActionIDs: []string{"postgresql.column.drop"}}.Map()},
				{Key: "indexes", Label: "Indexes", Icon: icon("key-round"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.table.indexes", Params: tableParams()}, Config: plugin.TableConfig{Columns: indexColumns(), ActionIDs: []string{"postgresql.index.create"}, RowActionIDs: []string{"postgresql.index.drop"}}.Map()},
				{Key: "constraints", Label: "Constraints", Icon: icon("shield-check"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.table.constraints", Params: tableParams()}, Config: plugin.TableConfig{Columns: constraintColumns()}.Map()},
				{Key: "ddl", Label: "DDL", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.table.ddl", Params: tableParams()}},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "postgresql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.scope}"}}, Config: queryConfig("SELECT * FROM ${resource.namespace}.${resource.name} LIMIT 100;", map[string]string{"database": "${resource.scope}"})},
			},
		},
	}
}

func viewResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "view", Title: "Views",
		List: plugin.DataSource{RouteID: "postgresql.views.list"}, Columns: viewColumns(),
		RowActionIDs: []string{"postgresql.view.drop"},
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}", ActionIDs: []string{"postgresql.view.drop"}}, Tabs: []plugin.Tab{
			{Key: "data", Label: "Data", Icon: icon("table"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "postgresql.view.rows", Params: tableParams()}},
			{Key: "definition", Label: "Definition", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.view.definition", Params: tableParams()}},
			{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "postgresql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.scope}"}}, Config: queryConfig("SELECT * FROM ${resource.namespace}.${resource.name} LIMIT 100;", map[string]string{"database": "${resource.scope}"})},
		}},
	}
}

func functionResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "function", Title: "Functions",
		List: plugin.DataSource{RouteID: "postgresql.functions.list"}, Columns: functionColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "definition", Label: "Definition", Icon: icon("code"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.function.definition", Params: map[string]string{"oid": "${resource.uid}", "database": "${resource.scope}"}}},
		}},
	}
}

func sequenceResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "sequence", Title: "Sequences",
		List: plugin.DataSource{RouteID: "postgresql.sequences.list"}, Columns: sequenceColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"}, Tabs: []plugin.Tab{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "postgresql.sequence.overview", Params: tableParams()}},
		}},
	}
}

// tableParams threads database/schema/table from the active resource into a
// table-scoped route. dataGridConfig reuses it for the editable Data grid.
func tableParams() map[string]string {
	return map[string]string{"database": "${resource.scope}", "schema": "${resource.namespace}", "table": "${resource.name}"}
}

func schemaParams() map[string]string {
	return map[string]string{"database": "${resource.scope}", "schema": "${resource.name}"}
}

func dataGridConfig() map[string]any {
	return plugin.TableConfig{
		Editable:      true,
		StagedEdits:   true,
		Exportable:    true,
		EmptyText:     "No rows.",
		Insert:        &plugin.DataSource{RouteID: "postgresql.table.row.insert", Method: plugin.MethodPost, Params: tableParams()},
		Update:        &plugin.DataSource{RouteID: "postgresql.table.row.update", Method: plugin.MethodPatch, Params: tableParams()},
		Delete:        &plugin.DataSource{RouteID: "postgresql.table.row.delete", Method: plugin.MethodDelete, Params: tableParams()},
		ColumnsSource: &plugin.DataSource{RouteID: "postgresql.table.columns", Params: tableParams()},
	}.Map()
}

func queryConfig(initial string, params map[string]string) map[string]any {
	cfg := plugin.QueryEditorConfig{
		Language:          "sql",
		Label:             "SQL",
		ExecuteLabel:      "Run query",
		CancelLabel:       "Cancel query",
		RunningLabel:      "Running…",
		EmptyText:         "Run a query to see results.",
		InitialQuery:      initial,
		CancelRouteID:     "postgresql.query.cancel",
		CompletionRouteID: "postgresql.completion",
		Exportable:        true,
	}
	if len(params) > 0 {
		cfg.CompletionParams = params
		cfg.CancelParams = params
	}
	return cfg.Map()
}

func databaseColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Database", Sortable: true},
		{Key: "owner", Label: "Owner", Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "encoding", Label: "Encoding"},
	}
}

func schemaColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Schema", Sortable: true}, {Key: "owner", Label: "Owner", Sortable: true}, {Key: "tables", Label: "Tables", Type: plugin.ColumnNumber, Sortable: true}, {Key: "views", Label: "Views", Type: plugin.ColumnNumber, Sortable: true}, {Key: "functions", Label: "Functions", Type: plugin.ColumnNumber, Sortable: true}}
}

func tableColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Table", Sortable: true}, {Key: "schema", Label: "Schema", Sortable: true}, {Key: "rows", Label: "Rows", Type: plugin.ColumnNumber, Sortable: true}, {Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true}, {Key: "owner", Label: "Owner", Sortable: true}}
}

func viewColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "View", Sortable: true}, {Key: "schema", Label: "Schema", Sortable: true}, {Key: "owner", Label: "Owner", Sortable: true}, {Key: "updatable", Label: "Updatable", Type: plugin.ColumnBool}}
}

func functionColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Function", Sortable: true}, {Key: "schema", Label: "Schema", Sortable: true}, {Key: "arguments", Label: "Arguments"}, {Key: "returns", Label: "Returns"}, {Key: "language", Label: "Language", Sortable: true}}
}

func sequenceColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Sequence", Sortable: true}, {Key: "schema", Label: "Schema", Sortable: true}, {Key: "dataType", Label: "Type"}, {Key: "start", Label: "Start", Type: plugin.ColumnNumber}, {Key: "increment", Label: "Increment", Type: plugin.ColumnNumber}}
}

func columnColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Column", Sortable: true}, {Key: "type", Label: "Type"}, {Key: "nullable", Label: "Nullable", Type: plugin.ColumnBool}, {Key: "default", Label: "Default"}, {Key: "identity", Label: "Identity"}, {Key: "position", Label: "Position", Type: plugin.ColumnNumber, Sortable: true}}
}

func indexColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Index", Sortable: true}, {Key: "unique", Label: "Unique", Type: plugin.ColumnBool}, {Key: "primary", Label: "Primary", Type: plugin.ColumnBool}, {Key: "definition", Label: "Definition"}}
}

func constraintColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Constraint", Sortable: true}, {Key: "type", Label: "Type", Sortable: true}, {Key: "definition", Label: "Definition"}}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: "postgresql.database.create", Label: "Create database", Icon: icon("plus"), RouteID: "postgresql.database.create", OnSuccess: &plugin.ActionSuccess{SelectTab: "schemas"}},
		{ID: "postgresql.database.drop", Label: "Drop database", Icon: icon("trash-2"), RouteID: "postgresql.database.drop", Params: map[string]string{"database": "${resource.uid}"}, Confirm: true, ConfirmText: "Drop this database? All of its schemas and data will be permanently deleted."},
		{ID: "postgresql.schema.create", Label: "Create schema", Icon: icon("folder-plus"), RouteID: "postgresql.schema.create", Params: map[string]string{"database": "${resource.uid}"}, OnSuccess: &plugin.ActionSuccess{SelectTab: "schemas"}},
		{ID: "postgresql.schema.drop", Label: "Drop schema", Icon: icon("trash-2"), RouteID: "postgresql.schema.drop", Params: schemaParams(), Confirm: true, ConfirmText: "Drop this schema? It must be empty."},
		{ID: "postgresql.table.create", Label: "Create table", Icon: icon("plus"), RouteID: "postgresql.table.create", Params: schemaParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "tables"}},
		{ID: "postgresql.column.add", Label: "Add column", Icon: icon("columns-3"), RouteID: "postgresql.column.add", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "postgresql.column.drop", Label: "Drop column", Icon: icon("trash"), RouteID: "postgresql.column.drop", Params: map[string]string{"schema": "${resource.scope}", "table": "${resource.namespace}", "name": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this column? Its data is permanently removed.", OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "postgresql.index.create", Label: "Create index", Icon: icon("plus"), RouteID: "postgresql.index.create", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: "postgresql.index.drop", Label: "Drop index", Icon: icon("trash"), RouteID: "postgresql.index.drop", Params: map[string]string{"schema": "${resource.scope}", "table": "${resource.namespace}", "name": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this index?", OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: "postgresql.table.truncate", Label: "Truncate", Icon: icon("eraser"), RouteID: "postgresql.table.truncate", Params: tableParams(), Confirm: true, ConfirmText: "Truncate this table? Every row will be deleted."},
		{ID: "postgresql.table.drop", Label: "Drop", Icon: icon("trash-2"), RouteID: "postgresql.table.drop", Params: tableParams(), Confirm: true, ConfirmText: "Drop this table? The table definition and data will be permanently deleted."},
		{ID: "postgresql.view.drop", Label: "Drop", Icon: icon("trash-2"), RouteID: "postgresql.view.drop", Params: map[string]string{"database": "${resource.scope}", "schema": "${resource.namespace}", "view": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this view?"},
	}
}
