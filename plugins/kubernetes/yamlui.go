package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

// yamlEditorConfig is the code_editor config that saves edits via server-side
// apply (POST). Used by the YAML detail tab and the Create dialog.
func yamlEditorConfig() plugin.CodeEditorConfig {
	return plugin.CodeEditorConfig{
		Language:    "yaml",
		SaveRouteID: "kubernetes.resource.apply",
		SaveMethod:  plugin.MethodPost,
	}
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
		Config: yamlEditorConfig(),
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
		Config: eventTimelineConfig(),
	}
}

func customResourceYAMLTab() plugin.Panel {
	return plugin.Panel{
		Key: "yaml", Label: "YAML", Icon: lucide("file-code"), Type: plugin.PanelCodeEditor,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.yaml", Params: map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}},
		Config: yamlEditorConfig(),
	}
}

func customResourceEventsTab() plugin.Panel {
	return plugin.Panel{
		Key: "events", Label: "Events", Icon: lucide("bell"), Type: plugin.PanelTimeline,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.events", Params: map[string]string{"kind": "${resource.scope}", "namespace": "${resource.namespace}", "name": "${resource.name}"}},
		Config: eventTimelineConfig(),
	}
}

func eventTimelineConfig() plugin.TimelineConfig {
	return plugin.TimelineConfig{
		TimestampField:    "createdAt",
		TitleField:        "reason",
		BodyField:         "message",
		SeverityField:     "type",
		ResourceField:     "object",
		EmptyText:         "No events.",
		RefreshIntervalMs: 10000,
	}
}

// createAction opens a dynamically-generated starter manifest for kind in a
// dialog; saving applies it. One per kind so the list's "Create" knows its kind.
func createAction(k kind) plugin.Action {
	return plugin.Action{
		ID: "kubernetes.create." + k.name, Label: "Create " + k.title, Icon: lucide("plus"),
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor,
		Params: map[string]string{"kind": k.name},
		Config: yamlEditorConfig(),
	}
}

// createCustomResourceAction is the single Create shared by every CRD list; the
// concrete kind isn't known until runtime, so it's supplied by the active list's
// scope params (not a static Params here). The template is derived from the CRD
// schema like any other kind.
func createCustomResourceAction() plugin.Action {
	return plugin.Action{
		ID: "kubernetes.create.customresource", Label: "Create", Icon: lucide("plus"),
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor,
		Config: yamlEditorConfig(),
	}
}
