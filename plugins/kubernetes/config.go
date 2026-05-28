package kubernetes

import "github.com/charlesng35/shellcn/internal/plugin"

// Metrics source options for cluster/node/workload overviews.
const (
	metricsServer = "metrics-server"
	metricsProm   = "prometheus"
	metricsNone   = "none"
)

// configSchema is the connection config form. Direct transport needs a
// kubeconfig (the cluster URL, CA, and credentials all live inside it); agent
// transport needs none, because the in-cluster agent injects the target's own
// ServiceAccount credentials. Namespace + metrics settings apply to both.
func configSchema() plugin.Schema {
	direct := plugin.Condition{AllOf: []plugin.Rule{
		{Field: plugin.SchemaContextTransport, Op: plugin.OpEq, Value: string(plugin.TransportDirect)},
	}}
	prom := plugin.Condition{AllOf: []plugin.Rule{
		{Field: "metrics_source", Op: plugin.OpEq, Value: metricsProm},
	}}
	return plugin.Schema{Groups: []plugin.Group{
		{
			Name: "Cluster",
			Fields: []plugin.Field{
				{
					Key: "kubeconfig", Label: "Kubeconfig", Type: plugin.FieldTextarea,
					Required: true, Secret: true, VisibleWhen: &direct,
					Placeholder: "apiVersion: v1\nkind: Config\nclusters: …",
					Help:        "Paste a kubeconfig. Its server URL, CA, and credentials are used to reach the cluster. Exec credential plugins are not allowed.",
				},
				{
					Key: "context", Label: "Context", Type: plugin.FieldText, VisibleWhen: &direct,
					Help: "Kubeconfig context to use. Blank uses the kubeconfig's current-context.",
				},
				{
					Key: "namespace", Label: "Default namespace", Type: plugin.FieldText,
					Help: "Namespace selected by default. Blank shows all namespaces.",
				},
			},
		},
		{
			Name: "Metrics",
			Fields: []plugin.Field{
				{
					Key: "metrics_source", Label: "Metrics source", Type: plugin.FieldSelect, Default: metricsServer,
					Options: []plugin.Option{
						{Label: "Metrics Server (metrics.k8s.io)", Value: metricsServer},
						{Label: "Prometheus", Value: metricsProm},
						{Label: "None", Value: metricsNone},
					},
					Help: "Source for live CPU/memory in overviews. Metrics Server is the cluster default.",
				},
				{
					Key: "prometheus_url", Label: "Prometheus URL", Type: plugin.FieldText, VisibleWhen: &prom,
					Placeholder: "http://prometheus.monitoring.svc:9090",
				},
			},
		},
	}}
}
