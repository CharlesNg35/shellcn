package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

// podRefParams are the resource identity params every pod stream needs.
func podRefParams(extra map[string]string) map[string]string {
	p := map[string]string{"namespace": "${resource.namespace}", "name": "${resource.name}"}
	for k, v := range extra {
		p[k] = v
	}
	return p
}

func runningPod() *plugin.Condition {
	return &plugin.Condition{AllOf: []plugin.Rule{{Field: "status", Op: plugin.OpEq, Value: "Running"}}}
}

// podDetailTabs adds workload diagnostics to the pod detail view.
func podDetailTabs() []plugin.Panel {
	return []plugin.Panel{
		{
			Key: "metrics", Label: "Metrics", Icon: lucide("activity"), Type: plugin.PanelMetrics,
			Source: &plugin.DataSource{
				RouteID: "kubernetes.pod.metrics", Method: plugin.MethodWS,
				Params: podRefParams(nil),
			},
			Config:      podMetricsConfig(),
			VisibleWhen: runningPod(),
		},
		{
			Key: "logs", Label: "Logs", Icon: lucide("scroll-text"), Type: plugin.PanelLogStream,
			Source: &plugin.DataSource{
				RouteID: "kubernetes.pod.logs", Method: plugin.MethodWS,
				Params: podRefParams(map[string]string{"follow": "true", "tail": "500", "timestamps": "true"}),
			},
			Config: plugin.LogStreamConfig{
				Controls: []plugin.StreamControl{{
					Param: "container", Label: "Container",
					OptionsSource: &plugin.DataSource{RouteID: "kubernetes.pod.containers", Params: podRefParams(map[string]string{"merge": "true"})},
				}},
				AllowPrevious: true,
			},
		},
		{
			Key: "terminal", Label: "Shell", Icon: lucide("terminal"), Type: plugin.PanelTerminal,
			Source: &plugin.DataSource{
				RouteID: "kubernetes.pod.exec", Method: plugin.MethodWS,
				Params: podRefParams(map[string]string{"tty": "true", "cols": "80", "rows": "24"}),
			},
			Config:      plugin.TerminalConfig{Zoom: true, Search: true},
			VisibleWhen: runningPod(),
		},
		podFilesTab(),
	}
}

// streams declares the pod log/terminal streams plus the cluster/node metrics
// streams the overview dashboards bind to.
func streams() []plugin.Stream {
	return []plugin.Stream{
		{ID: "kubernetes.resource.watch", Kind: plugin.StreamResource, RouteID: "kubernetes.resource.watch"},
		{ID: "kubernetes.resource.object.watch", Kind: plugin.StreamResource, RouteID: "kubernetes.resource.object.watch"},
		{ID: "kubernetes.resource.yaml.watch", Kind: plugin.StreamResource, RouteID: "kubernetes.resource.yaml.watch"},
		{ID: "kubernetes.resource.events.watch", Kind: plugin.StreamResource, RouteID: "kubernetes.resource.events.watch"},
		{ID: "kubernetes.pod.logs", Kind: plugin.StreamLogs, RouteID: "kubernetes.pod.logs"},
		{ID: "kubernetes.workload.logs", Kind: plugin.StreamLogs, RouteID: "kubernetes.workload.logs"},
		{ID: "kubernetes.pod.exec", Kind: plugin.StreamTerminal, RouteID: "kubernetes.pod.exec"},
		{ID: "kubernetes.cluster.shell", Kind: plugin.StreamTerminal, RouteID: "kubernetes.cluster.shell"},
		{ID: "kubernetes.cluster.metrics", Kind: plugin.StreamMetrics, RouteID: "kubernetes.cluster.metrics"},
		{ID: "kubernetes.node.metrics", Kind: plugin.StreamMetrics, RouteID: "kubernetes.node.metrics"},
		{ID: "kubernetes.pod.metrics", Kind: plugin.StreamMetrics, RouteID: "kubernetes.pod.metrics"},
	}
}

func podMetricsConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Stats: []plugin.MetricStat{
			{Key: "cpu", Label: "CPU", Unit: "cores"},
			{Key: "mem", Label: "Memory", Unit: "bytes"},
			{Key: "cpuRequest", Label: "CPU request", Unit: "cores"},
			{Key: "memRequest", Label: "Memory request", Unit: "bytes"},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU cores"},
			{Key: "mem", Label: "Memory", Unit: "bytes"},
		},
		History: 60,
	}
}

func podRecording() []plugin.RecordingCapability {
	return []plugin.RecordingCapability{{
		Class:         plugin.RecordingTerminal,
		Formats:       []plugin.RecordingFormat{plugin.FormatAsciicastV2},
		StreamIDs:     []string{"kubernetes.pod.exec", "kubernetes.cluster.shell"},
		Authoritative: true,
	}}
}
