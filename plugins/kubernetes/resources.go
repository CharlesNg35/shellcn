package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

// customResourceKind is the single generic ResourceType every CRD list reuses;
// the concrete GVR arrives as a list param, so one type renders all custom kinds.
const customResourceKind = "customresource"

func lucide(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

// namespaceScope is the global header selector that scopes every namespaced list
// to one namespace; options are the cluster's namespaces, empty means all.
func namespaceScope() plugin.ScopeFilter {
	return plugin.ScopeFilter{
		Param: "namespace", Label: "Namespace", Icon: lucide("layers"),
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

	tabs := []plugin.Panel{
		overviewTab(k, getParams),
		yamlTab(k),
	}
	tabs = append(tabs, k.detailTabs...)
	tabs = append(tabs, eventsTab(k))

	return plugin.ResourceType{
		Kind:    k.name,
		Title:   k.title,
		List:    plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": k.name}},
		Watch:   &plugin.DataSource{RouteID: "kubernetes.resource.watch", Method: plugin.MethodWS, Params: map[string]string{"kind": k.name}},
		Columns: k.columns,
		Actions: plugin.ResourceActions{
			Toolbar: []string{"kubernetes.create." + k.name},
			Row:     []string{"kubernetes.resource.delete"},
			Detail:  rowActions,
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: columnSeverities(k.columns, "status")},
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
		Actions: plugin.ResourceActions{
			Toolbar: []string{"kubernetes.create.customresource"},
			Row:     []string{"kubernetes.resource.delete"},
			Detail:  []string{"kubernetes.resource.delete"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{
					Key: "overview", Label: "Overview", Icon: lucide("info"), Type: plugin.PanelObjectDetail,
					Source: &plugin.DataSource{RouteID: "kubernetes.resource.overview", Params: map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}},
					Config: genericOverviewDetailConfig(),
				},
			},
		},
	}
}

func overviewTab(k kind, params map[string]string) plugin.Panel {
	return plugin.Panel{
		Key: "overview", Label: "Overview", Icon: lucide("info"), Type: plugin.PanelObjectDetail,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.overview", Params: params},
		Config: overviewDetailConfig(k),
	}
}

func actions() []plugin.Action {
	uid := map[string]string{"kind": "${resource.kind}", "namespace": "${resource.namespace}", "name": "${resource.name}"}
	base := []plugin.Action{
		{ID: "kubernetes.resource.delete", Label: "Delete", Icon: lucide("trash"), RouteID: "kubernetes.resource.delete", Params: uid, Confirm: true, ConfirmText: "Delete this resource?", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "kubernetes.resource.scale", Label: "Scale", Icon: lucide("move-vertical"), RouteID: "kubernetes.resource.scale", Params: uid},
		{ID: "kubernetes.resource.restart", Label: "Restart", Icon: lucide("refresh-cw"), RouteID: "kubernetes.resource.restart", Params: uid, Confirm: true, ConfirmText: "Roll out a restart?"},
		{ID: "kubernetes.node.cordon", Label: "Cordon", Icon: lucide("ban"), RouteID: "kubernetes.node.cordon", Params: uid, Confirm: true, ConfirmText: "Mark this node unschedulable?", EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpNeq, Value: true}}}, Group: "Scheduling"},
		{ID: "kubernetes.node.uncordon", Label: "Uncordon", Icon: lucide("circle-check"), RouteID: "kubernetes.node.uncordon", Params: uid, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpEq, Value: true}}}, Group: "Scheduling"},
		{ID: "kubernetes.node.drain", Label: "Drain", Icon: lucide("trash-2"), RouteID: "kubernetes.node.drain", Params: uid, Confirm: true, ConfirmText: "Cordon this node and evict its pods?", Group: "Scheduling"},
		{ID: "kubernetes.rollout.undo", Label: "Rollout undo", Icon: lucide("undo-2"), RouteID: "kubernetes.rollout.undo", Params: uid, Confirm: true, ConfirmText: "Roll back to the previous revision?"},
		{ID: "kubernetes.cronjob.trigger", Label: "Trigger", Icon: lucide("play"), RouteID: "kubernetes.cronjob.trigger", Params: uid, Confirm: true, ConfirmText: "Create a Job from this CronJob now?"},
		{ID: "kubernetes.service.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.service.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}}}},
		{ID: "kubernetes.pod.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.pod.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}}}},
	}
	base = append(base, clusterShellAction(), applyYAMLAction())
	for _, k := range kinds {
		base = append(base, createAction(k))
	}
	return append(base, createCustomResourceAction())
}
