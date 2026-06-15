// Package mysql implements the MySQL/MariaDB protocol plugin.
package mysql

import (
	"context"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

const mysqlIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128"><path fill="#00618A" d="M116.948 97.807c-6.863-.187-12.104.452-16.585 2.341-1.273.537-3.305.552-3.513 2.147.7.733.809 1.829 1.365 2.731 1.07 1.73 2.876 4.052 4.488 5.268 1.762 1.33 3.577 2.751 5.465 3.902 3.358 2.047 7.107 3.217 10.34 5.268 1.906 1.21 3.799 2.733 5.658 4.097.92.675 1.537 1.724 2.732 2.147v-.194c-.628-.8-.79-1.898-1.366-2.733l-2.537-2.537c-2.48-3.292-5.629-6.184-8.976-8.585-2.669-1.916-8.642-4.504-9.755-7.609l-.195-.195c1.892-.214 4.107-.898 5.854-1.367 2.934-.786 5.556-.583 8.585-1.365l4.097-1.171v-.78c-1.531-1.571-2.623-3.651-4.292-5.073-4.37-3.72-9.138-7.437-14.048-10.537-2.724-1.718-6.089-2.835-8.976-4.292-.971-.491-2.677-.746-3.318-1.562-1.517-1.932-2.342-4.382-3.511-6.633-2.449-4.717-4.854-9.868-7.024-14.831-1.48-3.384-2.447-6.72-4.293-9.756-8.86-14.567-18.396-23.358-33.169-32-3.144-1.838-6.929-2.563-10.929-3.513-2.145-.129-4.292-.26-6.438-.391-1.311-.546-2.673-2.149-3.902-2.927C17.811 4.565 5.257-2.16 1.633 6.682c-2.289 5.581 3.421 11.025 5.462 13.854 1.434 1.982 3.269 4.207 4.293 6.438.674 1.467.79 2.938 1.367 4.489 1.417 3.822 2.652 7.98 4.487 11.511.927 1.788 1.949 3.67 3.122 5.268.718.981 1.951 1.413 2.145 2.927-1.204 1.686-1.273 4.304-1.95 6.44-3.05 9.615-1.899 21.567 2.537 28.683 1.36 2.186 4.567 6.871 8.975 5.073 3.856-1.57 2.995-6.438 4.098-10.732.249-.973.096-1.689.585-2.341v.195l3.513 7.024c2.6 4.187 7.212 8.562 11.122 11.514 2.027 1.531 3.623 4.177 6.244 5.073v-.196h-.195c-.508-.791-1.303-1.119-1.951-1.755-1.527-1.497-3.225-3.358-4.487-5.073-3.556-4.827-6.698-10.11-9.561-15.609-1.368-2.627-2.557-5.523-3.709-8.196-.444-1.03-.438-2.589-1.364-3.122-1.263 1.958-3.122 3.542-4.098 5.854-1.561 3.696-1.762 8.204-2.341 12.878-.342.122-.19.038-.391.194-2.718-.655-3.672-3.452-4.683-5.853-2.554-6.07-3.029-15.842-.781-22.829.582-1.809 3.21-7.501 2.146-9.172-.508-1.666-2.184-2.63-3.121-3.903-1.161-1.574-2.319-3.646-3.124-5.464-2.09-4.731-3.066-10.044-5.267-14.828-1.053-2.287-2.832-4.602-4.293-6.634-1.617-2.253-3.429-3.912-4.683-6.635-.446-.968-1.051-2.518-.391-3.513.21-.671.508-.951 1.171-1.17 1.132-.873 4.284.29 5.462.779 3.129 1.3 5.741 2.538 8.392 4.294 1.271.844 2.559 2.475 4.097 2.927h1.756c2.747.631 5.824.195 8.391.975 4.536 1.378 8.601 3.523 12.292 5.854 11.246 7.102 20.442 17.21 26.732 29.269 1.012 1.942 1.45 3.794 2.341 5.854 1.798 4.153 4.063 8.426 5.852 12.488 1.786 4.052 3.526 8.141 6.05 11.513 1.327 1.772 6.451 2.723 8.781 3.708 1.632.689 4.307 1.409 5.854 2.34 2.953 1.782 5.815 3.903 8.586 5.855 1.383.975 5.64 3.116 5.852 4.879zM29.729 23.466c-1.431-.027-2.443.156-3.513.389v.195h.195c.683 1.402 1.888 2.306 2.731 3.513.65 1.367 1.301 2.732 1.952 4.097l.194-.193c1.209-.853 1.762-2.214 1.755-4.294-.484-.509-.555-1.147-.975-1.755-.556-.811-1.635-1.272-2.339-1.952z"/></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "MySQL / MariaDB",
		Description:         "MySQL and MariaDB cockpit with schema browser, table data, SQL editor, users, DDL helpers, and safety controls.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: mysqlIconSvg},
		Category:            plugin.CategoryDatabases,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"sql", "schema", "tables", "query_editor", "users"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSidebarTree,
		Tree:                tree(),
		Resources:           resources(),
		Actions:             actions(),
		Streams: []plugin.Stream{
			{ID: "mysql.query", Kind: plugin.StreamQuery, RouteID: "mysql.query"},
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
		{Key: "databases", Label: "Databases", Icon: icon("database"), Source: plugin.DataSource{RouteID: "mysql.databases.tree"}, Ref: &plugin.ResourceIdentity{Kind: "server", Name: "Databases", UID: "server"}},
		{Key: "users", Label: "Users", Icon: icon("users"), Source: plugin.DataSource{RouteID: "mysql.users.tree"}, ResourceKind: "user"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		serverResource(),
		databaseResource(),
		tableResource(),
		viewResource(),
		routineResource(),
		userResource(),
	}
}

// serverResource is the connection-level view opened by clicking the Databases
// tree group: the database list plus a SQL console, so a user can browse and run
// arbitrary SQL without first opening a specific database.
func serverResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "server", Title: "Databases",
		List: plugin.DataSource{RouteID: "mysql.databases.list"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Databases"},
			Tabs: []plugin.Panel{
				{Key: "databases", Label: "Databases", Icon: icon("database"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.databases.list"}, Config: databasesTableConfig([]string{"mysql.database.create"})},
				{Key: "console", Label: "SQL", Icon: icon("square-terminal"), Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "mysql.query", Method: plugin.MethodWS}, Config: queryConfig("SELECT VERSION();")},
			},
		},
	}
}

func databaseColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Database", Sortable: true},
		{Key: "charset", Label: "Charset"},
		{Key: "collation", Label: "Collation"},
		{Key: "tables", Label: "Tables", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "views", Label: "Views", Type: plugin.ColumnNumber, Sortable: true},
	}
}

func databaseResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "database", Title: "Databases",
		List:    plugin.DataSource{RouteID: "mysql.databases.list"},
		Columns: databaseColumns(),
		Actions: plugin.ResourceActions{
			Toolbar: []string{"mysql.database.create"},
			Row:     []string{"mysql.database.drop"},
			Detail:  []string{"mysql.database.drop"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("gauge"), Type: plugin.PanelDashboard, Config: databaseOverviewDashboard()},
				{Key: "tables", Label: "Tables", Icon: icon("table-2"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.tables.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: tablesTableConfig([]string{"mysql.table.create"}, []string{"mysql.table.truncate", "mysql.table.drop"})},
				{Key: "relations", Label: "Relationships", Icon: icon("workflow"), Type: plugin.PanelGraph, Source: &plugin.DataSource{RouteID: "mysql.relations.graph", Params: map[string]string{"database": "${resource.uid}"}}, Config: plugin.GraphConfig{Layout: plugin.GraphLayoutGrid, FitView: true}},
				{Key: "views", Label: "Views", Icon: icon("panel-top"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.views.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: viewsTableConfig([]string{"mysql.view.drop"})},
				{Key: "routines", Label: "Routines", Icon: icon("function-square"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.routines.list", Params: map[string]string{"database": "${resource.uid}"}}, Config: routinesTableConfig()},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "mysql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.uid}"}}, Config: queryConfig("SELECT VERSION();", map[string]string{"database": "${resource.uid}"})},
			},
		},
	}
}

func tableResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "table", Title: "Tables",
		List:    plugin.DataSource{RouteID: "mysql.tables.list"},
		Columns: tableColumns(),
		Actions: plugin.ResourceActions{
			Row:    []string{"mysql.table.truncate", "mysql.table.drop"},
			Detail: []string{"mysql.table.rename", "mysql.table.truncate", "mysql.table.drop"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "data", Label: "Data", Icon: icon("table"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.table.rows", Params: tableParams()}, Config: dataGridConfig()},
				{Key: "columns", Label: "Columns", Icon: icon("columns-3"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.table.columns", Params: tableParams()}, Config: columnsTableConfig([]string{"mysql.column.add"}, []string{"mysql.column.alter", "mysql.column.drop"})},
				{Key: "indexes", Label: "Indexes", Icon: icon("key-round"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.table.indexes", Params: tableParams()}, Config: indexesTableConfig([]string{"mysql.index.create"}, []string{"mysql.index.drop"})},
				{Key: "constraints", Label: "Constraints", Icon: icon("shield-check"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.table.constraints", Params: tableParams()}, Config: constraintsTableConfig([]string{"mysql.constraint.add"}, []string{"mysql.constraint.drop"})},
				{Key: "ddl", Label: "DDL", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "mysql.table.ddl", Params: tableParams()}},
				{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "mysql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.namespace}"}}, Config: queryConfig("SELECT * FROM `${resource.namespace}`.`${resource.name}` LIMIT 100;", map[string]string{"database": "${resource.namespace}"})},
			},
		},
	}
}

func viewResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "view", Title: "Views",
		List: plugin.DataSource{RouteID: "mysql.views.list"}, Columns: viewColumns(),
		Actions: plugin.ResourceActions{
			Row:    []string{"mysql.view.drop"},
			Detail: []string{"mysql.view.drop"},
		},
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"}, Tabs: []plugin.Panel{
			{Key: "data", Label: "Data", Icon: icon("table-properties"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "mysql.view.rows", Params: tableParams()}, Config: plugin.TableConfig{Exportable: true, EmptyText: "No rows returned."}},
			{Key: "definition", Label: "Definition", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "mysql.view.definition", Params: tableParams()}},
			{Key: "query", Label: "SQL", Icon: icon("square-terminal"), Type: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: "mysql.query", Method: plugin.MethodWS, Params: map[string]string{"database": "${resource.namespace}"}}, Config: queryConfig("SELECT * FROM `${resource.namespace}`.`${resource.name}` LIMIT 100;", map[string]string{"database": "${resource.namespace}"})},
		}},
	}
}

func routineResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "routine", Title: "Routines",
		List: plugin.DataSource{RouteID: "mysql.routines.list"}, Columns: routineColumns(),
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}"}, Tabs: []plugin.Panel{
			{Key: "definition", Label: "Definition", Icon: icon("code"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "mysql.routine.definition", Params: map[string]string{"id": "${resource.uid}"}}},
		}},
	}
}

func userResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "user", Title: "Users",
		List: plugin.DataSource{RouteID: "mysql.users.list"},
		Columns: []plugin.Column{
			{Key: "user", Label: "User", Sortable: true},
			{Key: "host", Label: "Host", Sortable: true},
			{Key: "plugin", Label: "Auth plugin"},
			{Key: "locked", Label: "Locked", Type: plugin.ColumnBool},
		},
		Actions: plugin.ResourceActions{
			Toolbar: []string{"mysql.user.create"},
			Row:     []string{"mysql.user.drop"},
			Detail:  []string{"mysql.user.grant", "mysql.user.drop"},
		},
		Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}"}, Tabs: []plugin.Panel{
			{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "mysql.user.overview", Params: map[string]string{"user": "${resource.name}", "host": "${resource.namespace}"}}, Config: userOverviewDetailConfig()},
		}},
	}
}

func databaseOverviewDashboard() plugin.DashboardConfig {
	databaseParams := map[string]string{"database": "${resource.uid}"}
	return plugin.DashboardConfig{Cells: []plugin.Panel{
		{Key: "summary", Label: "Summary", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "mysql.database.overview", Params: databaseParams}, Config: databaseOverviewDetailConfig(), Span: 2},
	}}
}

func databaseOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "Database", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "Name", Copy: true},
				{Key: "charset", Label: "Charset"},
				{Key: "collation", Label: "Collation"},
				{Key: "version", Label: "Server version"},
				{Key: "size", Label: "Size", Type: plugin.ColumnBytes},
				{Key: "tables", Label: "Tables", Type: plugin.ColumnNumber},
				{Key: "views", Label: "Views", Type: plugin.ColumnNumber},
			}},
		},
	}
}

func userOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		RawToggle: true,
		Sections: []plugin.ObjectDetailSection{
			{Title: "User", Fields: []plugin.ObjectDetailField{
				{Key: "user", Label: "User", Copy: true},
				{Key: "host", Label: "Host", Copy: true},
				{Key: "plugin", Label: "Auth plugin"},
				{Key: "locked", Label: "Locked", Type: plugin.ColumnBool},
			}},
		},
	}
}

func tableParams() map[string]string {
	return map[string]string{"database": "${resource.namespace}", "table": "${resource.name}"}
}

func dataGridConfig() plugin.TableConfig {
	return plugin.TableConfig{
		Editable:      true,
		StagedEdits:   true,
		Exportable:    true,
		EmptyText:     "No rows.",
		Insert:        &plugin.DataSource{RouteID: "mysql.table.row.insert", Method: plugin.MethodPost, Params: tableParams()},
		Update:        &plugin.DataSource{RouteID: "mysql.table.row.update", Method: plugin.MethodPatch, Params: tableParams()},
		Delete:        &plugin.DataSource{RouteID: "mysql.table.row.delete", Method: plugin.MethodDelete, Params: tableParams()},
		ColumnsSource: &plugin.DataSource{RouteID: "mysql.table.columns", Params: tableParams()},
	}
}

func queryConfig(initial string, params ...map[string]string) plugin.QueryEditorConfig {
	cfg := plugin.QueryEditorConfig{
		Language:          "mysql",
		Label:             "SQL",
		ExecuteLabel:      "Run query",
		CancelLabel:       "Cancel query",
		RunningLabel:      "Running...",
		EmptyText:         "Run a query to see results.",
		InitialQuery:      initial,
		CancelRouteID:     "mysql.query.cancel",
		CompletionRouteID: "mysql.completion",
		Exportable:        true,
	}
	if len(params) > 0 && len(params[0]) > 0 {
		cfg.CancelParams = params[0]
		cfg.CompletionParams = params[0]
	}
	return cfg
}

func databasesTableConfig(actions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: databaseColumns(), ActionIDs: actions, EmptyText: "No databases visible.", Exportable: true}
}

func tablesTableConfig(actions, rowActions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: tableColumns(), ActionIDs: actions, RowActionIDs: rowActions, EmptyText: "No tables.", Exportable: true}
}

func viewsTableConfig(rowActions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: viewColumns(), RowActionIDs: rowActions, EmptyText: "No views.", Exportable: true}
}

func routinesTableConfig() plugin.TableConfig {
	return plugin.TableConfig{Columns: routineColumns(), EmptyText: "No routines.", Exportable: true}
}

func columnsTableConfig(actions, rowActions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: columnColumns(), ActionIDs: actions, RowActionIDs: rowActions, EmptyText: "No columns.", Exportable: true}
}

func indexesTableConfig(actions, rowActions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: indexColumns(), ActionIDs: actions, RowActionIDs: rowActions, EmptyText: "No indexes.", Exportable: true}
}

func constraintsTableConfig(actions, rowActions []string) plugin.TableConfig {
	return plugin.TableConfig{Columns: constraintColumns(), ActionIDs: actions, RowActionIDs: rowActions, EmptyText: "No constraints.", Exportable: true}
}

func tableColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Table", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "engine", Label: "Engine"}, {Key: "rows", Label: "Rows", Type: plugin.ColumnNumber, Sortable: true}, {Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true}, {Key: "collation", Label: "Collation"}}
}

func viewColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "View", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "definer", Label: "Definer"}, {Key: "updatable", Label: "Updatable", Type: plugin.ColumnBool}}
}

func routineColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Routine", Sortable: true}, {Key: "database", Label: "Database", Sortable: true}, {Key: "type", Label: "Type", Sortable: true}, {Key: "returns", Label: "Returns"}, {Key: "definer", Label: "Definer"}, {Key: "modified", Label: "Modified"}}
}

func columnColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Column", Sortable: true}, {Key: "type", Label: "Type"}, {Key: "nullable", Label: "Nullable", Type: plugin.ColumnBool}, {Key: "default", Label: "Default"}, {Key: "extra", Label: "Extra"}, {Key: "position", Label: "Position", Type: plugin.ColumnNumber, Sortable: true}}
}

func indexColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Index", Sortable: true}, {Key: "column", Label: "Column"}, {Key: "unique", Label: "Unique", Type: plugin.ColumnBool}, {Key: "type", Label: "Type"}, {Key: "sequence", Label: "Seq", Type: plugin.ColumnNumber}}
}

func constraintColumns() []plugin.Column {
	return []plugin.Column{{Key: "name", Label: "Constraint", Sortable: true}, {Key: "type", Label: "Type", Sortable: true}, {Key: "column", Label: "Column"}, {Key: "referenced", Label: "Referenced"}}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: "mysql.database.create", Label: "Create database", Icon: icon("plus"), RouteID: "mysql.database.create"},
		{ID: "mysql.table.create", Label: "Create table", Icon: icon("plus"), RouteID: "mysql.table.create", Params: map[string]string{"database": "${resource.uid}"}, OnSuccess: &plugin.ActionSuccess{SelectTab: "tables"}},
		{ID: "mysql.column.add", Label: "Add column", Icon: icon("columns-3"), RouteID: "mysql.column.add", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "mysql.column.drop", Label: "Drop column", Icon: icon("trash"), RouteID: "mysql.column.drop", Params: map[string]string{"database": "${record.database}", "table": "${record.table}", "name": "${record.name}"}, Confirm: true, ConfirmText: "Drop this column? Its data is permanently removed.", OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "mysql.index.create", Label: "Create index", Icon: icon("plus"), RouteID: "mysql.index.create", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: "mysql.index.drop", Label: "Drop index", Icon: icon("trash"), RouteID: "mysql.index.drop", Params: map[string]string{"database": "${record.database}", "table": "${record.table}", "name": "${record.name}"}, Confirm: true, ConfirmText: "Drop this index?", OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: "mysql.table.truncate", Label: "Truncate", Icon: icon("eraser"), RouteID: "mysql.table.truncate", Params: tableParams(), Confirm: true, ConfirmText: "Truncate this table? Every row will be deleted.", Group: "Danger zone"},
		{ID: "mysql.table.drop", Label: "Drop table", Icon: icon("trash-2"), RouteID: "mysql.table.drop", Params: tableParams(), Confirm: true, ConfirmText: "Drop this table? The table definition and data will be permanently deleted.", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}, Group: "Danger zone"},
		{ID: "mysql.table.rename", Label: "Rename table", Icon: icon("pencil"), RouteID: "mysql.table.rename", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "mysql.view.drop", Label: "Drop view", Icon: icon("trash-2"), RouteID: "mysql.view.drop", Params: map[string]string{"database": "${resource.namespace}", "view": "${resource.name}"}, Confirm: true, ConfirmText: "Drop this view?", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "mysql.database.drop", Label: "Drop database", Icon: icon("trash-2"), RouteID: "mysql.database.drop", Params: map[string]string{"database": "${resource.uid}"}, Confirm: true, ConfirmText: "Drop this database? All of its tables, views, and data will be permanently deleted.", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "mysql.constraint.add", Label: "Add constraint", Icon: icon("plus"), RouteID: "mysql.constraint.add", Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "constraints"}},
		{ID: "mysql.constraint.drop", Label: "Drop constraint", Icon: icon("trash"), RouteID: "mysql.constraint.drop", Params: map[string]string{"database": "${record.database}", "table": "${record.table}", "name": "${record.name}", "type": "${record.type}"}, Confirm: true, ConfirmText: "Drop this constraint?", OnSuccess: &plugin.ActionSuccess{SelectTab: "constraints"}},
		{ID: "mysql.column.alter", Label: "Alter column", Icon: icon("pencil"), RouteID: "mysql.column.alter", Params: map[string]string{"database": "${record.database}", "table": "${record.table}", "name": "${record.name}"}, OnSuccess: &plugin.ActionSuccess{SelectTab: "columns"}},
		{ID: "mysql.user.create", Label: "Create user", Icon: icon("user-plus"), RouteID: "mysql.user.create", Confirm: true, ConfirmText: "Create this MySQL user?", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "mysql.user.grant", Label: "Grant privileges", Icon: icon("shield-plus"), RouteID: "mysql.user.grant", Params: map[string]string{"user": "${resource.name}", "host": "${resource.namespace}"}, Confirm: true, ConfirmText: "Grant these privileges to this MySQL user?"},
		{ID: "mysql.user.drop", Label: "Drop user", Icon: icon("trash-2"), RouteID: "mysql.user.drop", Params: map[string]string{"user": "${resource.name}", "host": "${resource.namespace}"}, Confirm: true, ConfirmText: "Drop this user? Their privileges are permanently removed.", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}, Group: "Danger zone"},
	}
}
