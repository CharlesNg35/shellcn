package meilisearch

import "github.com/charlesng/shellcn/internal/plugin"

func icon(name string) plugin.Icon {
	return plugin.Icon{Type: plugin.IconLucide, Value: name}
}

func rid(suffix string) string { return "meilisearch." + suffix }

func tree() []plugin.TreeGroup {
	return []plugin.TreeGroup{
		{Key: "indexes", Label: "Indexes", Icon: icon("database"), Source: plugin.DataSource{RouteID: rid("indexes.tree")}, ResourceKind: "index"},
		{Key: "tasks", Label: "Tasks", Icon: icon("list-checks"), Source: plugin.DataSource{RouteID: rid("tasks.tree")}, ResourceKind: "task"},
		{Key: "keys", Label: "API keys", Icon: icon("key-round"), Source: plugin.DataSource{RouteID: rid("keys.tree")}, ResourceKind: "key"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{
		{
			Kind: "index", Title: "Indexes", List: plugin.DataSource{RouteID: rid("indexes.list")},
			Columns:       indexColumns(),
			ListActionIDs: []string{rid("index.create"), rid("dump.create"), rid("snapshot.create")},
			RowActionIDs:  []string{rid("index.delete")},
			Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{
				rid("settings.update"), rid("index.update"), rid("documents.delete_all"), rid("index.delete"),
			}}, Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("index.overview"), Params: indexParams()}},
				{Key: "documents", Label: "Documents", Icon: icon("file-json"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("documents.list"), Params: indexParams()}, Config: plugin.TableConfig{Columns: documentColumns(), ActionIDs: []string{rid("document.upsert")}, RowActionIDs: []string{rid("document.delete")}, Exportable: true}.Map()},
				{Key: "search", Label: "Search", Icon: icon("search"), Panel: plugin.PanelQueryEditor, Source: &plugin.DataSource{RouteID: rid("search.query"), Method: plugin.MethodWS, Params: indexParams()}, Config: searchConfig()},
				{Key: "settings", Label: "Settings", Icon: icon("settings"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("settings.read"), Params: indexParams()}},
				{Key: "stats", Label: "Stats", Icon: icon("chart-column"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("index.stats"), Params: indexParams()}},
				{Key: "tasks", Label: "Tasks", Icon: icon("list-checks"), Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: rid("tasks.list"), Params: indexParams()}, Config: plugin.TableConfig{Columns: taskColumns(), RowActionIDs: []string{rid("task.cancel")}, Exportable: true}.Map()},
			}},
		},
		{
			Kind: "document", Title: "Documents", List: plugin.DataSource{RouteID: rid("documents.list")},
			Columns:      documentColumns(),
			RowActionIDs: []string{rid("document.delete")},
			Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.namespace}/${resource.name}", ActionIDs: []string{rid("document.delete")}}, Tabs: []plugin.Tab{
				{Key: "document", Label: "Document", Icon: icon("file-json"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("document.read"), Params: documentParams()}},
				{Key: "editor", Label: "Editor", Icon: icon("code"), Panel: plugin.PanelCodeEditor, Source: &plugin.DataSource{RouteID: rid("document.read"), Params: documentParams()}, Config: map[string]any{"language": "json", "saveRouteId": rid("document.update"), "saveMethod": "PUT", "saveParams": documentParams()}},
			}},
		},
		{
			Kind: "task", Title: "Tasks", List: plugin.DataSource{RouteID: rid("tasks.list")},
			Columns:      taskColumns(),
			RowActionIDs: []string{rid("task.cancel")},
			Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "Task ${resource.name}", StatusField: "status", ActionIDs: []string{rid("task.cancel")}}, Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("task.read"), Params: taskParams()}},
			}},
		},
		{
			Kind: "key", Title: "API keys", List: plugin.DataSource{RouteID: rid("keys.list")},
			Columns:       keyColumns(),
			ListActionIDs: []string{rid("key.create")},
			RowActionIDs:  []string{rid("key.delete")},
			Detail: plugin.DetailView{Header: plugin.HeaderSpec{Title: "${resource.name}", ActionIDs: []string{rid("key.delete")}}, Tabs: []plugin.Tab{
				{Key: "overview", Label: "Overview", Icon: icon("key-round"), Panel: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: rid("key.read"), Params: keyParams()}},
			}},
		},
	}
}

func actions() []plugin.Action {
	return []plugin.Action{
		{ID: rid("index.create"), Label: "Create index", Icon: icon("plus"), RouteID: rid("index.create")},
		{ID: rid("index.update"), Label: "Update primary key", Icon: icon("key"), RouteID: rid("index.update"), Params: indexParams(), Confirm: true, ConfirmText: "Update this index primary key?"},
		{ID: rid("index.delete"), Label: "Delete", Icon: icon("trash-2"), RouteID: rid("index.delete"), Params: indexParams(), Confirm: true, ConfirmText: "Delete this index and its documents?"},
		{ID: rid("settings.update"), Label: "Update settings", Icon: icon("settings"), RouteID: rid("settings.update"), Params: indexParams(), Confirm: true, ConfirmText: "Update this index settings?"},
		{ID: rid("document.upsert"), Label: "Upsert document", Icon: icon("plus"), RouteID: rid("document.upsert"), Params: indexParams()},
		{ID: rid("document.delete"), Label: "Delete", Icon: icon("trash"), RouteID: rid("document.delete"), Params: documentParams(), Confirm: true, ConfirmText: "Delete this document?"},
		{ID: rid("documents.delete_all"), Label: "Delete all documents", Icon: icon("eraser"), RouteID: rid("documents.delete_all"), Params: indexParams(), Confirm: true, ConfirmText: "Delete every document in this index?"},
		{ID: rid("task.cancel"), Label: "Cancel", Icon: icon("ban"), RouteID: rid("task.cancel"), Params: taskParams(), Confirm: true, ConfirmText: "Cancel matching enqueued or processing tasks?"},
		{ID: rid("key.create"), Label: "Create key", Icon: icon("plus"), RouteID: rid("key.create")},
		{ID: rid("key.delete"), Label: "Delete", Icon: icon("trash"), RouteID: rid("key.delete"), Params: keyParams(), Confirm: true, ConfirmText: "Delete this API key?"},
		{ID: rid("dump.create"), Label: "Create dump", Icon: icon("archive"), RouteID: rid("dump.create"), Confirm: true, ConfirmText: "Create a Meilisearch dump?"},
		{ID: rid("snapshot.create"), Label: "Create snapshot", Icon: icon("camera"), RouteID: rid("snapshot.create"), Confirm: true, ConfirmText: "Create a Meilisearch snapshot?"},
	}
}

func searchConfig() map[string]any {
	return map[string]any{
		"language":          "json",
		"label":             "Meilisearch query",
		"executeLabel":      "Search",
		"runningLabel":      "Searching...",
		"emptyText":         "Run a Meilisearch JSON search to see hits.",
		"initialQuery":      `{"q":"","limit":50}`,
		"completionRouteId": rid("completion"),
		"exportable":        true,
	}
}

func indexParams() map[string]string { return map[string]string{"index": "${resource.name}"} }
func taskParams() map[string]string  { return map[string]string{"task": "${resource.name}"} }
func keyParams() map[string]string   { return map[string]string{"key": "${resource.name}"} }
func documentParams() map[string]string {
	return map[string]string{"index": "${resource.namespace}", "id": "${resource.name}"}
}

func indexColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "uid", Label: "Index", Sortable: true},
		{Key: "primaryKey", Label: "Primary key", Sortable: true},
		{Key: "numberOfDocuments", Label: "Documents", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "isIndexing", Label: "Indexing", Type: plugin.ColumnBool},
		{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "updatedAt", Label: "Updated", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func documentColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "_id", Label: "ID", Sortable: true},
		{Key: "_index", Label: "Index", Sortable: true},
		{Key: "_source", Label: "Document", Type: plugin.ColumnJSON},
	}
}

func taskColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "uid", Label: "UID", Type: plugin.ColumnNumber, Sortable: true},
		{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "indexUid", Label: "Index", Sortable: true},
		{Key: "duration", Label: "Duration"},
		{Key: "enqueuedAt", Label: "Enqueued", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "startedAt", Label: "Started", Type: plugin.ColumnDateTime},
		{Key: "finishedAt", Label: "Finished", Type: plugin.ColumnDateTime},
	}
}

func keyColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "uid", Label: "UID", Sortable: true},
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "description", Label: "Description"},
		{Key: "actions", Label: "Actions", Type: plugin.ColumnJSON},
		{Key: "indexes", Label: "Indexes", Type: plugin.ColumnJSON},
		{Key: "expiresAt", Label: "Expires", Type: plugin.ColumnDateTime, Sortable: true},
	}
}
