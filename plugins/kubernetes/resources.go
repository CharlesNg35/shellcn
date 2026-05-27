package kubernetes

import "github.com/charlesng/shellcn/internal/plugin"

// customResourceKind is the single generic ResourceType every CRD list reuses;
// the concrete GVR arrives as a list param, so one type renders all custom kinds.
const customResourceKind = "customresource"

func lucide(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

func resources() []plugin.ResourceType {
	out := make([]plugin.ResourceType, 0, len(kinds)+2)
	out = append(out, clusterResourceType())
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
	// Every kind gets Edit YAML + its specific actions; the list gets Create.
	rowActions := append([]string{"kubernetes.resource.edit"}, k.actionIDs...)

	tabs := []plugin.Tab{
		{
			Key: "overview", Label: "Overview", Icon: lucide("info"), Panel: plugin.PanelDocument,
			Source: &plugin.DataSource{RouteID: "kubernetes.resource.get", Params: getParams},
		},
		yamlTab(k),
	}
	tabs = append(tabs, k.detailTabs...)
	tabs = append(tabs, eventsTab(k))

	return plugin.ResourceType{
		Kind:          k.name,
		Title:         k.title,
		List:          plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": k.name}},
		Watch:         &plugin.DataSource{RouteID: "kubernetes.resource.watch", Method: plugin.MethodWS, Params: map[string]string{"kind": k.name}},
		Columns:       k.columns,
		ActionIDs:     rowActions,
		ListActionIDs: []string{"kubernetes.create." + k.name},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", ActionIDs: rowActions},
			Tabs:   tabs,
		},
	}
}

// customResourceType renders any CRD instance list/detail. The CRD's GVR travels
// in the list params (and per-row "scope"), so one type covers every custom kind.
func customResourceType() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    customResourceKind,
		Title:   "Custom Resource",
		List:    plugin.DataSource{RouteID: "kubernetes.resource.list"},
		Columns: []plugin.Column{nameCol(), nsCol(), ageCol()},
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
		{ID: "kubernetes.resource.delete", Label: "Delete", Icon: lucide("trash"), RouteID: "kubernetes.resource.delete", Params: uid, Confirm: true, ConfirmText: "Delete this resource?"},
		{ID: "kubernetes.resource.scale", Label: "Scale", Icon: lucide("move-vertical"), RouteID: "kubernetes.resource.scale", Params: uid},
		{ID: "kubernetes.resource.restart", Label: "Restart", Icon: lucide("refresh-cw"), RouteID: "kubernetes.resource.restart", Params: uid, Confirm: true, ConfirmText: "Roll out a restart?"},
		{ID: "kubernetes.node.cordon", Label: "Cordon", Icon: lucide("ban"), RouteID: "kubernetes.node.cordon", Params: uid, Confirm: true, ConfirmText: "Mark this node unschedulable?"},
		{ID: "kubernetes.node.uncordon", Label: "Uncordon", Icon: lucide("circle-check"), RouteID: "kubernetes.node.uncordon", Params: uid},
		editAction(),
	}
	base = append(base, podActions()...)
	for _, k := range kinds {
		base = append(base, createAction(k))
	}
	return base
}
