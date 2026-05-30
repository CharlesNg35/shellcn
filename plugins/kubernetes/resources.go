package kubernetes

import "github.com/charlesng35/shellcn/internal/plugin"

// customResourceKind is the single generic ResourceType every CRD list reuses;
// the concrete GVR arrives as a list param, so one type renders all custom kinds.
const customResourceKind = "customresource"

func lucide(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

// namespaceFilter is the list toolbar selector that scopes a namespaced kind to a
// single namespace; options are the cluster's namespaces, empty means all.
func namespaceFilter() plugin.ResourceFilter {
	return plugin.ResourceFilter{
		Key: "namespace", Label: "Namespace", Param: "namespace",
		OptionsSource: &plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": "namespace"}},
		ValueField:    "name",
		AllLabel:      "All namespaces",
	}
}

func resources() []plugin.ResourceType {
	out := make([]plugin.ResourceType, 0, len(kinds)+3)
	out = append(out, clusterResourceType(), helmReleaseResourceType())
	for _, k := range kinds {
		out = append(out, resourceType(k))
	}
	return append(out, customResourceType())
}

func resourceType(k kind) plugin.ResourceType {
	getParams := map[string]string{"kind": k.name, "name": "${resource.name}"}
	if k.namespaced {
		getParams["namespace"] = "${resource.namespace}"
	}
	// Edit and create live in the YAML tab / a dialog, not as row buttons.
	rowActions := append([]string(nil), k.actionIDs...)

	tabs := []plugin.Tab{
		{
			Key: "overview", Label: "Overview", Icon: lucide("info"), Panel: plugin.PanelDocument,
			Source: &plugin.DataSource{RouteID: "kubernetes.resource.get", Params: getParams},
		},
		yamlTab(k),
	}
	tabs = append(tabs, k.detailTabs...)
	tabs = append(tabs, eventsTab(k))

	var filters []plugin.ResourceFilter
	if k.namespaced {
		filters = []plugin.ResourceFilter{namespaceFilter()}
	}

	return plugin.ResourceType{
		Kind:          k.name,
		Title:         k.title,
		List:          plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": k.name}},
		Watch:         &plugin.DataSource{RouteID: "kubernetes.resource.watch", Method: plugin.MethodWS, Params: map[string]string{"kind": k.name}},
		Columns:       k.columns,
		Filters:       filters,
		ActionIDs:     rowActions,
		ListActionIDs: []string{"kubernetes.create." + k.name},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: columnSeverities(k.columns, "status"), ActionIDs: rowActions},
			Tabs:   tabs,
		},
	}
}

// customResourceType renders any CRD instance list/detail. The CRD's GVR travels
// in the list params (and per-row "scope"), so one type covers every custom kind.
func customResourceType() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:  customResourceKind,
		Title: "Custom Resource",
		List:  plugin.DataSource{RouteID: "kubernetes.resource.list"},
		// No static columns: the CRD's own printer columns are fetched at runtime.
		ColumnsSource: &plugin.DataSource{RouteID: "kubernetes.resource.columns"},
		ListActionIDs: []string{"kubernetes.create.customresource"},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Tab{
				{
					Key: "overview", Label: "Overview", Icon: lucide("info"), Panel: plugin.PanelDocument,
					Source: &plugin.DataSource{RouteID: "kubernetes.resource.get", Params: map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}},
				},
			},
		},
	}
}

func actions() []plugin.Action {
	uid := map[string]string{"kind": "${resource.kind}", "namespace": "${resource.namespace}", "name": "${resource.name}"}
	base := []plugin.Action{
		{ID: "kubernetes.resource.delete", Label: "Delete", Icon: lucide("trash"), RouteID: "kubernetes.resource.delete", Params: uid, Confirm: true, ConfirmText: "Delete this resource?", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "kubernetes.resource.scale", Label: "Scale", Icon: lucide("move-vertical"), RouteID: "kubernetes.resource.scale", Params: uid},
		{ID: "kubernetes.resource.restart", Label: "Restart", Icon: lucide("refresh-cw"), RouteID: "kubernetes.resource.restart", Params: uid, Confirm: true, ConfirmText: "Roll out a restart?"},
		{ID: "kubernetes.node.cordon", Label: "Cordon", Icon: lucide("ban"), RouteID: "kubernetes.node.cordon", Params: uid, Confirm: true, ConfirmText: "Mark this node unschedulable?", EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpNeq, Value: true}}}},
		{ID: "kubernetes.node.uncordon", Label: "Uncordon", Icon: lucide("circle-check"), RouteID: "kubernetes.node.uncordon", Params: uid, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpEq, Value: true}}}},
		{ID: "kubernetes.service.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.service.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}}}},
		{ID: "kubernetes.pod.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.pod.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}}}},
	}
	for _, k := range kinds {
		base = append(base, createAction(k))
	}
	return append(base, createCustomResourceAction())
}
