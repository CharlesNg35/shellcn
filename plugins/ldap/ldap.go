// Package ldap implements the LDAP directory protocol plugin.
package ldap

import (
	"context"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

const ldapIconSvg = `<svg xmlns="http://www.w3.org/2000/svg" xml:space="preserve" viewBox="0 0 512 512"><path d="M412.3 395.8h-238l119-206.1zm3.6-28.2H512L390.5 159.8l-47.6 81.3zm-199.5-79.2 45.1-78.1-55-94.1-45 77zm-10 17.3-72.5-125.6-12.4-21.1L0 366.8h171.1z" style="fill:#cb2026"/></svg>`

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "LDAP",
		Description:         "Directory browser: navigate the DIT, view and edit entry attributes inline, add/rename/delete entries, and run subtree searches.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: ldapIconSvg},
		Category:            plugin.CategorySecurity,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"directory", "search", "data_grid"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSidebarTree,
		Tree:                tree(),
		Resources:           resources(),
		Actions:             actions(),
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
		{Key: "directory", Label: "Directory", Icon: icon("folder-tree"), Source: plugin.DataSource{RouteID: "ldap.tree.root"}, ResourceKind: "entry"},
	}
}

func resources() []plugin.ResourceType {
	return []plugin.ResourceType{entryResource()}
}

func entryResource() plugin.ResourceType {
	dnParams := map[string]string{"dn": "${resource.uid}"}
	return plugin.ResourceType{
		Kind: "entry", Title: "Entries",
		List:    plugin.DataSource{RouteID: "ldap.entries.search"},
		Columns: entryColumns(),
		Actions: plugin.ResourceActions{
			Row:    []string{"ldap.entry.delete"},
			Detail: []string{"ldap.entry.add", "ldap.entry.rename", "ldap.entry.delete"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "entryType", Severities: map[string]plugin.Severity{
				"domain": plugin.SeverityInfo, "container": plugin.SeverityInfo,
				"group": plugin.SeveritySuccess, "person": plugin.SeveritySecondary,
				"computer": plugin.SeverityWarn, "entry": plugin.SeveritySecondary,
			}},
			Tabs: []plugin.Panel{
				{Key: "overview", Label: "Overview", Icon: icon("info"), Type: plugin.PanelObjectDetail, Source: &plugin.DataSource{RouteID: "ldap.entry.overview", Params: dnParams}, Config: entryOverviewConfig()},
				{Key: "attributes", Label: "Attributes", Icon: icon("table"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ldap.entry.attributes", Params: dnParams}, Config: attributeGridConfig(dnParams)},
				{Key: "children", Label: "Children", Icon: icon("folder-tree"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ldap.entry.children", Params: dnParams}, Config: plugin.TableConfig{Columns: entryColumns(), RowActionIDs: []string{"ldap.entry.delete"}, EmptyText: "No child entries.", Exportable: true, RowClick: plugin.RowClickNavigate}},
				{Key: "subtree", Label: "Subtree search", Icon: icon("search"), Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ldap.entries.search", Params: map[string]string{"base": "${resource.uid}"}}, Config: plugin.TableConfig{Columns: entryColumns(), DefaultSort: &plugin.SortKey{Field: "dn"}, EmptyText: "No matching entries. Use the table search box for a name fragment or LDAP filter.", Exportable: true, RowClick: plugin.RowClickNavigate}},
				{Key: "ldif", Label: "LDIF", Icon: icon("file-text"), Type: plugin.PanelDocument, Source: &plugin.DataSource{RouteID: "ldap.entry.ldif", Params: dnParams}},
			},
		},
	}
}

func entryOverviewConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "Identity", Fields: []plugin.ObjectDetailField{
				{Key: "name", Label: "RDN", Copy: true},
				{Key: "dn", Label: "DN", Copy: true},
				{Key: "parent", Label: "Parent DN", Copy: true},
				{Key: "entryType", Label: "Type", Type: plugin.ColumnBadge, Severities: map[string]plugin.Severity{
					"domain": plugin.SeverityInfo, "container": plugin.SeverityInfo,
					"group": plugin.SeveritySuccess, "person": plugin.SeveritySecondary,
					"computer": plugin.SeverityWarn, "entry": plugin.SeveritySecondary,
				}},
			}},
			{Title: "Directory", Fields: []plugin.ObjectDetailField{
				{Key: "objectClass", Label: "Object classes"},
				{Key: "attributeCount", Label: "Attributes", Type: plugin.ColumnNumber},
				{Key: "hasChildren", Label: "Has children", Type: plugin.ColumnBool},
				{Key: "readOnly", Label: "Read-only", Type: plugin.ColumnBool},
			}},
		},
		RawToggle: true,
	}
}

func attributeGridConfig(dnParams map[string]string) plugin.TableConfig {
	return plugin.TableConfig{
		Columns:     attributeColumns(),
		Editable:    true,
		StagedEdits: true,
		Exportable:  true,
		RowKey:      []string{"attribute"},
		EmptyText:   "No attributes.",
		Insert:      &plugin.DataSource{RouteID: "ldap.entry.attr.add", Method: plugin.MethodPost, Params: dnParams},
		Update:      &plugin.DataSource{RouteID: "ldap.entry.attr.update", Method: plugin.MethodPatch, Params: dnParams},
		Delete:      &plugin.DataSource{RouteID: "ldap.entry.attr.delete", Method: plugin.MethodDelete, Params: dnParams},
	}
}

func entryColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "icon", Label: "", Type: plugin.ColumnIcon, Width: "3rem"},
		{Key: "name", Label: "RDN", Sortable: true},
		{Key: "dn", Label: "DN", Sortable: true},
		{Key: "parent", Label: "Parent DN", Sortable: true},
		{Key: "entryType", Label: "Type", Type: plugin.ColumnBadge, Sortable: true, Severities: map[string]plugin.Severity{
			"domain": plugin.SeverityInfo, "container": plugin.SeverityInfo,
			"group": plugin.SeveritySuccess, "person": plugin.SeveritySecondary,
			"computer": plugin.SeverityWarn, "entry": plugin.SeveritySecondary,
		}},
		{Key: "hasChildren", Label: "Children", Type: plugin.ColumnBool, Sortable: true},
		{Key: "objectClass", Label: "Object class"},
	}
}

func attributeColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "attribute", Label: "Attribute", Sortable: true, ReadOnly: true},
		{Key: "value", Label: "Value", Editable: true, Editor: plugin.ColumnEditorText},
	}
}

func actions() []plugin.Action {
	writable := &plugin.Condition{AllOf: []plugin.Rule{{Field: "readOnly", Op: plugin.OpNeq, Value: true}}}
	return []plugin.Action{
		{ID: "ldap.entry.add", Label: "Add child entry", Icon: icon("plus"), RouteID: "ldap.entry.add", Params: map[string]string{"parent": "${resource.uid}"}, EnabledWhen: writable, OnSuccess: &plugin.ActionSuccess{SelectTab: "children"}},
		{ID: "ldap.entry.rename", Label: "Rename or move", Icon: icon("pencil"), RouteID: "ldap.entry.rename", Params: map[string]string{"dn": "${resource.uid}"}, Confirm: true, ConfirmText: "Rename or move this entry? Existing references to its DN may need to be updated.", EnabledWhen: writable},
		{ID: "ldap.entry.delete", Label: "Delete entry", Icon: icon("trash-2"), RouteID: "ldap.entry.delete", Params: map[string]string{"dn": "${resource.uid}"}, Confirm: true, ConfirmText: "Delete this entry? This permanently removes it from the directory.", EnabledWhen: writable, OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
	}
}
