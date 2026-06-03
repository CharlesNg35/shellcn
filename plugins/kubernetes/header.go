package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

// headerActionIDs are the connection-wide affordances pinned to the workspace
// header center.
func headerActionIDs() []string {
	return []string{"kubernetes.cluster.shell", "kubernetes.cluster.apply"}
}

// clusterShellAction docks a terminal attached to a freshly launched kubectl pod.
func clusterShellAction() plugin.Action {
	return plugin.Action{
		ID: "kubernetes.cluster.shell", Label: "Cluster Shell", Icon: lucide("terminal"),
		RouteID: "kubernetes.cluster.shell", Open: plugin.OpenDock, Panel: plugin.PanelTerminal,
		Params:   map[string]string{"tty": "true", "cols": "80", "rows": "24"},
		Config:   plugin.TerminalConfig{Zoom: true, Search: true},
		IconOnly: true,
	}
}

// applyYAMLAction opens a blank editor whose Save server-side-applies the manifest.
func applyYAMLAction() plugin.Action {
	return plugin.Action{
		ID: "kubernetes.cluster.apply", Label: "Apply YAML", Icon: lucide("file-up"),
		RouteID: "kubernetes.resource.apply", Open: plugin.OpenDialog, Panel: plugin.PanelCodeEditor,
		Config: plugin.CodeEditorConfig{
			Language:       "yaml",
			InitialContent: applyStarter,
			SaveRouteID:    "kubernetes.resource.apply",
			SaveMethod:     plugin.MethodPost,
		},
		IconOnly: true,
	}
}

const applyStarter = `# Paste or edit manifests, then Save to apply them to the cluster.
# Separate multiple documents with "---".
# apiVersion: v1
# kind: ConfigMap
# metadata:
#   name: example
#   namespace: default
# data:
#   key: value
`
