package kubernetes

import "github.com/charlesng/shellcn/internal/plugin"

const (
	permRead   = "kubernetes.resources.read"
	permWrite  = "kubernetes.resources.write"
	permDelete = "kubernetes.resources.delete"
)

// Routes wires the generic, catalog-driven Kubernetes routes. One set of routes
// (parameterized by {kind}) serves every built-in kind and every CRD.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "kubernetes.tree.category", Method: plugin.MethodGet, Path: "/tree/category/{category}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.category", Handle: TreeCategory},
		{ID: "kubernetes.tree.kind", Method: plugin.MethodGet, Path: "/tree/kind/{kind}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.kind", Handle: TreeKindInstances},
		{ID: "kubernetes.tree.crds", Method: plugin.MethodGet, Path: "/tree/crds", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.crds", Handle: TreeCRDs},

		{ID: "kubernetes.resource.list", Method: plugin.MethodGet, Path: "/resources/{kind}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.list", Handle: ListResource},
		{ID: "kubernetes.resource.get", Method: plugin.MethodGet, Path: "/resources/{kind}/get", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.get", Handle: GetResource},
		{ID: "kubernetes.resource.watch", Method: plugin.MethodWS, Path: "/resources/{kind}/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.watch", Stream: WatchResource},

		{ID: "kubernetes.resource.delete", Method: plugin.MethodDelete, Path: "/resources/{kind}/delete", Permission: permDelete, Risk: plugin.RiskDestructive, AuditEvent: "kubernetes.resource.delete", Handle: DeleteResource},
		{ID: "kubernetes.resource.scale", Method: plugin.MethodPost, Path: "/resources/{kind}/scale", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.scale", Input: scaleSchema(), Handle: ScaleResource},
		{ID: "kubernetes.resource.restart", Method: plugin.MethodPost, Path: "/resources/{kind}/restart", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.restart", Handle: RestartResource},

		{ID: "kubernetes.node.cordon", Method: plugin.MethodPost, Path: "/nodes/cordon", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.node.cordon", Handle: CordonNode},
		{ID: "kubernetes.node.uncordon", Method: plugin.MethodPost, Path: "/nodes/uncordon", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.node.uncordon", Handle: UncordonNode},

		{ID: "kubernetes.resource.yaml", Method: plugin.MethodGet, Path: "/resources/{kind}/yaml", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.yaml", Handle: GetYAML},
		{ID: "kubernetes.resource.template", Method: plugin.MethodGet, Path: "/resources/{kind}/template", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.template", Handle: TemplateYAML},
		{ID: "kubernetes.resource.apply", Method: plugin.MethodPost, Path: "/apply", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.apply", Input: applySchema(), Handle: ApplyYAML},
		{ID: "kubernetes.resource.events", Method: plugin.MethodGet, Path: "/resources/{kind}/events", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.events", Handle: ResourceEvents},

		{ID: "kubernetes.pod.logs", Method: plugin.MethodWS, Path: "/pods/logs", Permission: "kubernetes.pods.logs", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.logs", Stream: LogsStream},
		{ID: "kubernetes.pod.exec", Method: plugin.MethodWS, Path: "/pods/exec", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.exec", Stream: ExecStream},
		{ID: "kubernetes.pod.portforward", Method: plugin.MethodWS, Path: "/pods/portforward", Permission: "kubernetes.pods.portforward", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.portforward", Stream: PortForwardStream},

		{ID: "kubernetes.cluster.tree", Method: plugin.MethodGet, Path: "/tree/overview", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.cluster.tree", Handle: ClusterTree},
		{ID: "kubernetes.cluster.list", Method: plugin.MethodGet, Path: "/overview", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.cluster.list", Handle: ClusterList},
		{ID: "kubernetes.cluster.metrics", Method: plugin.MethodWS, Path: "/overview/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.cluster.metrics", Stream: ClusterMetrics},
		{ID: "kubernetes.node.metrics", Method: plugin.MethodWS, Path: "/nodes/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.node.metrics", Stream: NodeMetrics},
		{ID: "kubernetes.node.pods", Method: plugin.MethodGet, Path: "/nodes/pods", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.node.pods", Handle: NodePods},
		{ID: "kubernetes.workload.pods", Method: plugin.MethodGet, Path: "/resources/{kind}/pods", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.workload.pods", Handle: WorkloadPods},
	}
}

func sess(rc *plugin.RequestContext) (*Session, error) { return Unwrap(rc.Session) }

func applySchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Apply",
		Fields: []plugin.Field{
			{Key: "content", Label: "Manifest", Type: plugin.FieldTextarea, Required: true},
			{Key: "dryRun", Label: "Dry run", Type: plugin.FieldToggle},
		},
	}}}
}

func scaleSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Scale",
		Fields: []plugin.Field{
			{Key: "replicas", Label: "Replicas", Type: plugin.FieldNumber, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}}},
		},
	}}}
}
