package kubernetes

import "github.com/charlesng/shellcn/internal/plugin"

// yamlEditorConfig is the code_editor config that saves edits via server-side
// apply (POST). Used by the YAML detail tab and the Edit/Create dock actions.
func yamlEditorConfig() map[string]any {
	return map[string]any{
		"language":    "yaml",
		"saveRouteId": "kubernetes.resource.apply",
		"saveMethod":  "POST",
	}
}

// yamlTab is the editable YAML detail tab (loads current object, applies on save).
func yamlTab(k kind) plugin.Tab {
	getParams := map[string]string{"kind": k.name, "name": "${resource.name}"}
	if k.namespaced {
		getParams["namespace"] = "${resource.namespace}"
	}
	return plugin.Tab{
		Key: "yaml", Label: "YAML", Icon: lucide("file-code"), Panel: plugin.PanelCodeEditor,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.yaml", Params: getParams},
		Config: yamlEditorConfig(),
	}
}

// eventsTab shows the events involving an object.
func eventsTab(k kind) plugin.Tab {
	params := map[string]string{"kind": k.name, "name": "${resource.name}"}
	if k.namespaced {
		params["namespace"] = "${resource.namespace}"
	}
	return plugin.Tab{
		Key: "events", Label: "Events", Icon: lucide("bell"), Panel: plugin.PanelTable,
		Source: &plugin.DataSource{RouteID: "kubernetes.resource.events", Params: params},
		Config: plugin.TableConfig{Columns: []plugin.Column{
			col("type", "Type", badge), col("reason", "Reason"), col("message", "Message", notSort), col("count", "Count", num), ageCol(),
		}}.Map(),
	}
}

// editAction opens the resource's YAML in the dock for editing.
func editAction() plugin.Action {
	return plugin.Action{
		ID: "kubernetes.resource.edit", Label: "Edit YAML", Icon: lucide("file-code"),
		RouteID: "kubernetes.resource.yaml", Open: plugin.OpenDock, Panel: plugin.PanelCodeEditor,
		Params: map[string]string{"kind": "${resource.kind}", "namespace": "${resource.namespace}", "name": "${resource.name}"},
		Config: yamlEditorConfig(),
	}
}

// createAction opens a dynamically-generated starter manifest for kind in the
// dock; saving applies it. One per kind so the list's "Create" knows its kind.
func createAction(k kind) plugin.Action {
	return plugin.Action{
		ID: "kubernetes.create." + k.name, Label: "Create " + k.title, Icon: lucide("plus"),
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDock, Panel: plugin.PanelCodeEditor,
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
		RouteID: "kubernetes.resource.template", Open: plugin.OpenDock, Panel: plugin.PanelCodeEditor,
		Config: yamlEditorConfig(),
	}
}
