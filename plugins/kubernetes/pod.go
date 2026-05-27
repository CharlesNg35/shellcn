package kubernetes

import "github.com/charlesng/shellcn/internal/plugin"

// podRefParams are the resource identity params every pod stream needs.
func podRefParams(extra map[string]string) map[string]string {
	p := map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

// podDetailTabs adds Logs and Shell tabs to the pod detail view.
func podDetailTabs() []plugin.Tab {
	return []plugin.Tab{
		{
			Key: "logs", Label: "Logs", Icon: lucide("scroll-text"), Panel: plugin.PanelLogStream,
			Source: &plugin.DataSource{
				RouteID: "kubernetes.pod.logs", Method: plugin.MethodWS,
				Params: podRefParams(map[string]string{"follow": "true", "tail": "500", "timestamps": "true"}),
			},
		},
		{
			Key: "terminal", Label: "Shell", Icon: lucide("terminal"), Panel: plugin.PanelTerminal,
			Source: &plugin.DataSource{
				RouteID: "kubernetes.pod.exec", Method: plugin.MethodWS,
				Params: podRefParams(map[string]string{"command": "/bin/sh", "tty": "true", "cols": "80", "rows": "24"}),
			},
			Config: plugin.TerminalConfig{Zoom: true, Search: true}.Map(),
		},
	}
}

// podActions are the dock-opening pod actions (Logs/Shell) plus port-forward.
func podActions() []plugin.Action {
	return []plugin.Action{
		{
			ID: "kubernetes.pod.logs", Label: "Logs", Icon: lucide("scroll-text"),
			RouteID: "kubernetes.pod.logs", Open: plugin.OpenDock, Panel: plugin.PanelLogStream,
			Params: podRefParams(map[string]string{"follow": "true", "tail": "500", "timestamps": "true"}),
		},
		{
			ID: "kubernetes.pod.exec", Label: "Shell", Icon: lucide("terminal"),
			RouteID: "kubernetes.pod.exec", Open: plugin.OpenDock, Panel: plugin.PanelTerminal,
			Params: podRefParams(map[string]string{"command": "/bin/sh", "tty": "true", "cols": "80", "rows": "24"}),
		},
	}
}

// streams declares the pod log/terminal streams plus the cluster/node metrics
// streams the overview dashboards bind to.
func streams() []plugin.Stream {
	return []plugin.Stream{
		{ID: "kubernetes.pod.logs", Kind: plugin.StreamLogs, RouteID: "kubernetes.pod.logs"},
		{ID: "kubernetes.pod.exec", Kind: plugin.StreamTerminal, RouteID: "kubernetes.pod.exec"},
		{ID: "kubernetes.cluster.metrics", Kind: plugin.StreamMetrics, RouteID: "kubernetes.cluster.metrics"},
		{ID: "kubernetes.node.metrics", Kind: plugin.StreamMetrics, RouteID: "kubernetes.node.metrics"},
	}
}

func podRecording() []plugin.RecordingCapability {
	return []plugin.RecordingCapability{{
		Class:         plugin.RecordingTerminal,
		Formats:       []plugin.RecordingFormat{plugin.FormatAsciicastV2},
		StreamIDs:     []string{"kubernetes.pod.exec"},
		Authoritative: true,
	}}
}
