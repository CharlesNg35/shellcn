package dynamodb

import "github.com/charlesng35/shellcn/internal/plugin"

func tree() []plugin.TreeGroup {
	return []plugin.TreeGroup{
		{Key: "tables", Label: "Tables", Icon: icon("table-2"), Source: plugin.DataSource{RouteID: rid("tables.tree")}, ResourceKind: "table"},
		{Key: "backups", Label: "Backups", Icon: icon("archive"), Source: plugin.DataSource{RouteID: rid("backups.tree")}, ResourceKind: "backup"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		tableResource(),
		indexResource(),
		itemResource(),
		backupResource(),
	}
}

func tableResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "table", Title: "Tables",
		List:         plugin.DataSource{RouteID: rid("tables.list")},
		Columns:      tableColumns(),
		ActionIDs:    []string{rid("table.create")},
		RowActionIDs: []string{rid("table.delete"), rid("backup.create")},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{rid("table.delete")}},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("table.read"), Params: tableParams()}},
				{Key: "items", Label: "Items", Icon: icon("braces"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("items.list"), Params: tableParams()}, Config: plugin.TableConfig{Exportable: true, ActionIDs: []string{rid("item.put")}, RowActionIDs: []string{rid("item.delete")}, EmptyText: "No items found."}.Map()},
				{Key: "indexes", Label: "Indexes", Icon: icon("list-tree"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("indexes.list"), Params: tableParams()}, Config: plugin.TableConfig{Columns: indexColumns(), Exportable: true, ActionIDs: []string{rid("gsi.create")}, RowActionIDs: []string{rid("gsi.delete")}}.Map()},
				{Key: "capacity", Label: "Capacity", Icon: icon("gauge"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("table.capacity"), Params: tableParams()}},
				{Key: "ttl", Label: "TTL", Icon: icon("timer"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("ttl.read"), Params: tableParams()}},
				{Key: "tags", Label: "Tags", Icon: icon("tags"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("tags.list"), Params: tableParams()}, Config: plugin.TableConfig{Columns: tagColumns(), Exportable: true}.Map()},
				{Key: "backups", Label: "Backups", Icon: icon("archive"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("backups.list"), Params: tableParams()}, Config: plugin.TableConfig{Columns: backupColumns(), Exportable: true, ActionIDs: []string{rid("backup.create")}, RowActionIDs: []string{rid("backup.delete")}}.Map()},
				{Key: "partiql", Label: "PartiQL", Icon: icon("square-terminal"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: rid("partiql"), Method: plugin.MethodWS, Params: tableParams()}, Config: queryConfig(`SELECT * FROM "${resource.name}" LIMIT 25`)},
			},
		},
	}
}

func indexResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "index", Title: "Indexes",
		List:         plugin.DataSource{RouteID: rid("indexes.list")},
		Columns:      indexColumns(),
		RowActionIDs: []string{rid("gsi.delete")},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.namespace}.${resource.name}", ActionIDs: []string{rid("gsi.delete")}},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("index.read"), Params: indexParams()}},
			},
		},
	}
}

func itemResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "item", Title: "Items",
		List:         plugin.DataSource{RouteID: rid("items.list")},
		RowActionIDs: []string{rid("item.delete")},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{rid("item.delete")}},
			Tabs: []plugin.Tab{
				{Key: "document", Label: "Item", Icon: icon("braces"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("item.read"), Params: map[string]string{"id": "${resource.uid}"}}},
				{Key: "editor", Label: "Editor", Icon: icon("code"), Panel: plugin.PanelCodeEditor, Source: &plugin.DataSource{RouteID: rid("item.read"), Params: map[string]string{"id": "${resource.uid}"}}, Config: map[string]any{"language": "json", "saveRouteId": rid("item.update"), "saveMethod": "PUT", "saveParams": map[string]string{"id": "${resource.uid}"}}},
			},
		},
	}
}

func backupResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind: "backup", Title: "Backups",
		List:         plugin.DataSource{RouteID: rid("backups.list")},
		Columns:      backupColumns(),
		RowActionIDs: []string{rid("backup.delete")},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{rid("backup.delete")}},
			Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("backup.read"), Params: map[string]string{"backup": "${resource.uid}"}}},
			},
		},
	}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: rid("table.create"), Label: "Create table", Icon: icon("plus"), RouteID: rid("table.create"), Confirm: true},
		{ID: rid("table.delete"), Label: "Delete table", Icon: icon("trash-2"), RouteID: rid("table.delete"), Params: tableParams(), Confirm: true, ConfirmText: "Delete this DynamoDB table and all items?"},
		{ID: rid("item.put"), Label: "Put item", Icon: icon("plus"), RouteID: rid("item.put"), Params: tableParams(), OnSuccess: &plugin.ActionSuccess{SelectTab: "items"}},
		{ID: rid("item.delete"), Label: "Delete item", Icon: icon("trash"), RouteID: rid("item.delete"), Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Delete this item?"},
		{ID: rid("gsi.create"), Label: "Create GSI", Icon: icon("plus"), RouteID: rid("gsi.create"), Params: tableParams(), Confirm: true, OnSuccess: &plugin.ActionSuccess{SelectTab: "indexes"}},
		{ID: rid("gsi.delete"), Label: "Delete GSI", Icon: icon("trash"), RouteID: rid("gsi.delete"), Params: indexParams(), Confirm: true, ConfirmText: "Delete this global secondary index?"},
		{ID: rid("backup.create"), Label: "Create backup", Icon: icon("archive"), RouteID: rid("backup.create"), Params: tableParams(), Confirm: true, OnSuccess: &plugin.ActionSuccess{SelectTab: "backups"}},
		{ID: rid("backup.delete"), Label: "Delete backup", Icon: icon("trash"), RouteID: rid("backup.delete"), Params: map[string]string{"backup": "${resource.uid}"}, Confirm: true, ConfirmText: "Delete this backup?"},
		{ID: rid("ttl.update"), Label: "Update TTL", Icon: icon("timer-reset"), RouteID: rid("ttl.update"), Params: tableParams(), Confirm: true, OnSuccess: &plugin.ActionSuccess{SelectTab: "ttl"}},
	}
}

func queryConfig(initial string) map[string]any {
	return map[string]any{
		"language":          "sql",
		"label":             "PartiQL",
		"executeLabel":      "Run",
		"runningLabel":      "Running...",
		"emptyText":         "Run a PartiQL statement to see results.",
		"initialQuery":      initial,
		"completionRouteId": rid("completion"),
		"exportable":        true,
	}
}

func tableParams() map[string]string { return map[string]string{"table": "${resource.name}"} }
func indexParams() map[string]string {
	return map[string]string{"table": "${resource.namespace}", "index": "${resource.name}"}
}

func tableColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Table", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "billing_mode", Label: "Billing", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "items", Label: "Items", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "created", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func indexColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Index", Sortable: true},
		{Key: "table", Label: "Table", Sortable: true},
		{Key: "kind", Label: "Kind", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "key_schema", Label: "Key schema"},
		{Key: "projection", Label: "Projection", Type: plugin.ColumnBadge},
		{Key: "items", Label: "Items", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
	}
}

func backupColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Backup", Sortable: true},
		{Key: "table", Label: "Table", Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "size", Label: "Size", Type: plugin.ColumnBytes, Sortable: true},
		{Key: "created", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func tagColumns() []plugin.Column {
	return []plugin.Column{{Key: "key", Label: "Key", Sortable: true}, {Key: "value", Label: "Value"}}
}
