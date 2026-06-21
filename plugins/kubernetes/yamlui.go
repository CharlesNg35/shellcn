package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

// yamlEditorConfig is the code_editor config that saves edits via server-side
// apply (POST). RefreshField resets the editor to the canonical applied object;
// watch (when set) live-updates the content. Used by the YAML detail tab and the
// Create dialog (which has no live source).
func yamlEditorConfig(watch *plugin.DataSource) plugin.CodeEditorConfig {
	return plugin.CodeEditorConfig{
		Language:     "yaml",
		SaveRouteID:  "kubernetes.resource.apply",
		SaveMethod:   plugin.MethodPost,
		RefreshField: "content",
		DryRunKey:    "dryRun",
		Watch:        watch,
		SaveToast:    &plugin.SaveToast{Summary: "Applied"},
	}
}

func yamlWatchSource(params map[string]string) *plugin.DataSource {
	return &plugin.DataSource{RouteID: "kubernetes.resource.yaml.watch", Method: plugin.MethodWS, Params: params}
}

// yamlTab is the editable YAML detail tab (loads current object, applies on save).
func yamlTab(k kind) plugin.Panel {
	getParams := map[string]string{"kind": k.name, "name": "${resource.name}"}
	if k.namespaced {
		getParams["namespace"] = "${resource.namespace}"
	}
	return plugin.Panel{
		Key: "yaml", Label: "YAML", Icon: lucide("file-code"), Type: plugin.PanelCodeEditor,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.yaml", Params: getParams},
		Config: yamlEditorConfig(yamlWatchSource(getParams)),
	}
}

func eventsTab(k kind) plugin.Panel {
	params := map[string]string{"kind": k.name, "name": "${resource.name}"}
	if k.namespaced {
		params["namespace"] = "${resource.namespace}"
	}
	return plugin.Panel{
		Key: "events", Label: "Events", Icon: lucide("bell"), Type: plugin.PanelTimeline,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.events", Params: params},
		Config: eventTimelineConfig(eventsWatchSource(params)),
	}
}

func eventsWatchSource(params map[string]string) *plugin.DataSource {
	return &plugin.DataSource{RouteID: "kubernetes.resource.events.watch", Method: plugin.MethodWS, Params: params}
}

func customResourceYAMLTab() plugin.Panel {
	params := map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}
	return plugin.Panel{
		Key: "yaml", Label: "YAML", Icon: lucide("file-code"), Type: plugin.PanelCodeEditor,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.yaml", Params: params},
		Config: yamlEditorConfig(yamlWatchSource(params)),
	}
}

func customResourceEventsTab() plugin.Panel {
	params := map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}
	return plugin.Panel{
		Key: "events", Label: "Events", Icon: lucide("bell"), Type: plugin.PanelTimeline,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.events", Params: params},
		Config: eventTimelineConfig(eventsWatchSource(params)),
	}
}

// eventTimelineConfig live-watches object-scoped events; with no watch source (the
// cluster-wide feed) it falls back to periodic refresh.
func eventTimelineConfig(watch *plugin.DataSource) plugin.TimelineConfig {
	cfg := plugin.TimelineConfig{
		TimestampField: "createdAt",
		TitleField:     "reason",
		BodyField:      "message",
		SeverityField:  "type",
		ResourceField:  "object",
		EmptyText:      "No events.",
	}
	if watch != nil {
		cfg.Watch = watch
	} else {
		cfg.RefreshIntervalMs = 10000
	}
	return cfg
}

// createAction opens a dynamically-generated starter manifest for kind in a
// dialog; saving applies it. One per kind so the list's "Create" knows its kind.
func createAction(k kind) plugin.Action {
	cfg := yamlEditorConfig(nil)
	cfg.SaveToast = &plugin.SaveToast{Summary: "Created " + k.title}
	cfg.SaveDismiss = plugin.SaveDismissClose
	return plugin.Action{
		ID: "kubernetes.create." + k.name, Label: "Create " + k.title, Icon: lucide("plus"),
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor,
		Params: map[string]string{"kind": k.name},
		Config: cfg,
	}
}

// createCustomResourceAction is the single Create shared by every CRD list; the
// concrete kind isn't known until runtime, so it's supplied by the active list's
// scope params (not a static Params here). The template is derived from the CRD
// schema like any other kind.
func createCustomResourceAction() plugin.Action {
	cfg := yamlEditorConfig(nil)
	cfg.SaveToast = &plugin.SaveToast{Summary: "Created"}
	cfg.SaveDismiss = plugin.SaveDismissClose
	return plugin.Action{
		ID: "kubernetes.create.customresource", Label: "Create", Icon: lucide("plus"),
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor,
		Config: cfg,
	}
}
