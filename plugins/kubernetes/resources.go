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
		Param: "namespace", Label: "Namespace", Icon: lucide("layers"), Control: plugin.ScopeSelect,
		OptionsSource: &plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": "namespace"}},
		WatchSource:   &plugin.DataSource{RouteID: "kubernetes.resource.watch", Method: plugin.MethodWS, Params: map[string]string{"kind": "namespace"}},
		ValueField:    "name",
		LabelField:    "name",
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
	toolbarActions := []string{}
	if !k.noCreate {
		toolbarActions = append(toolbarActions, "kubernetes.create."+k.name)
	}
	rowActions := []string{}
	if !k.noDelete {
		rowActions = append(rowActions, "kubernetes.resource.delete")
	}
	detailActions := detailActionIDs(k)

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
			Toolbar: toolbarActions,
			Row:     rowActions,
			Detail:  detailActions,
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "status", Severities: columnSeverities(k.columns, "status")},
			Tabs:   tabs,
		},
	}
}

func detailActionIDs(k kind) []string {
	out := make([]string, 0, len(k.actionIDs))
	for _, id := range k.actionIDs {
		if k.noDelete && id == "kubernetes.resource.delete" {
			continue
		}
		out = append(out, id)
	}
	return out
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
			Row:     []string{"kubernetes.customresource.delete"},
			Detail:  []string{"kubernetes.customresource.delete"},
		},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{
					Key: "overview", Label: "Overview", Icon: lucide("info"), Type: plugin.PanelObjectDetail,
					Source: &plugin.DataSource{RouteID: "kubernetes.resource.overview", Params: map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}},
					Config: genericOverviewDetailConfig(),
				},
				customResourceYAMLTab(),
				customResourceEventsTab(),
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
	customUID := map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}
	base := []plugin.Action{
		{ID: "kubernetes.resource.delete", Label: "Delete", Icon: lucide("trash"), RouteID: "kubernetes.resource.delete", Params: uid, Confirm: true, ConfirmText: "Delete this Kubernetes resource? Kubernetes may also delete dependent objects through owner references and finalizers.", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "kubernetes.customresource.delete", Label: "Delete", Icon: lucide("trash"), RouteID: "kubernetes.resource.delete", Params: customUID, Confirm: true, ConfirmText: "Delete this custom resource? Kubernetes may also delete dependent objects through owner references and finalizers.", OnSuccess: &plugin.ActionSuccess{Navigate: plugin.NavigateList}},
		{ID: "kubernetes.resource.scale", Label: "Scale replicas", Icon: lucide("move-vertical"), RouteID: "kubernetes.resource.scale", Params: uid},
		{ID: "kubernetes.resource.restart", Label: "Restart rollout", Icon: lucide("refresh-cw"), RouteID: "kubernetes.resource.restart", Params: uid, Confirm: true, ConfirmText: "Restart this workload by updating its pod template? This starts a rolling replacement of matching Pods."},
		{ID: "kubernetes.node.cordon", Label: "Cordon", Icon: lucide("ban"), RouteID: "kubernetes.node.cordon", Params: uid, Confirm: true, ConfirmText: "Mark this node unschedulable?", EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpNeq, Value: true}}}, Group: "Scheduling"},
		{ID: "kubernetes.node.uncordon", Label: "Uncordon", Icon: lucide("circle-check"), RouteID: "kubernetes.node.uncordon", Params: uid, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "unschedulable", Op: plugin.OpEq, Value: true}}}, Group: "Scheduling"},
		{ID: "kubernetes.node.drain", Label: "Drain", Icon: lucide("trash-2"), RouteID: "kubernetes.node.drain", Params: uid, Confirm: true, ConfirmText: "Cordon this node and evict eligible Pods? DaemonSet and mirror Pods are skipped; PodDisruptionBudgets can block eviction.", Group: "Scheduling"},
		{ID: "kubernetes.rollout.undo", Label: "Undo rollout", Icon: lucide("undo-2"), RouteID: "kubernetes.rollout.undo", Params: uid, Confirm: true, ConfirmText: "Roll back this Deployment to its previous ReplicaSet revision?"},
		{ID: "kubernetes.cronjob.trigger", Label: "Trigger now", Icon: lucide("play"), RouteID: "kubernetes.cronjob.trigger", Params: uid, Confirm: true, ConfirmText: "Create a one-off Job from this CronJob now? This does not change the schedule."},
		{ID: "kubernetes.service.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.service.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}, {Field: "type", Op: plugin.OpNeq, Value: "ExternalName"}}}},
		{ID: "kubernetes.pod.open", Label: "Open", Icon: lucide("external-link"), RouteID: "kubernetes.pod.open", Open: plugin.OpenURL, Params: map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}, EnabledWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "ports", Op: plugin.OpNotEmpty}, {Field: "status", Op: plugin.OpEq, Value: "Running"}}}},
	}
	base = append(base, clusterShellAction(), applyYAMLAction())
	for _, k := range kinds {
		base = append(base, createAction(k))
	}
	return append(base, createCustomResourceAction())
}
