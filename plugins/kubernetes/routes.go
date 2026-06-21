package kubernetes

import "github.com/charlesng35/shellcn/sdk/plugin"

const (
	permRead         = "kubernetes.resources.read"
	permWrite        = "kubernetes.resources.write"
	permDelete       = "kubernetes.resources.delete"
	permClusterShell = "kubernetes.cluster.shell"
)

// Routes wires the generic, catalog-driven Kubernetes routes. One set of routes
// (parameterized by {kind}) serves every built-in kind and every CRD.
func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "kubernetes.tree.category", Method: plugin.MethodGet, Path: "/tree/category/{category}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.category", Handle: TreeCategory},
		{ID: "kubernetes.tree.crds", Method: plugin.MethodGet, Path: "/tree/crds", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.crds", Handle: TreeCRDs},
		{ID: "kubernetes.tree.crdgroup", Method: plugin.MethodGet, Path: "/tree/crd-group", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.crdgroup", Handle: TreeCRDGroup},
		{ID: "kubernetes.tree.subgroup", Method: plugin.MethodGet, Path: "/tree/subgroup/{subgroup}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.subgroup", Handle: TreeSubgroup},
		{ID: "kubernetes.tree.gatewayapi", Method: plugin.MethodGet, Path: "/tree/gatewayapi", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.gatewayapi", Handle: TreeGatewayAPI},

		{ID: "kubernetes.resource.list", Method: plugin.MethodGet, Path: "/resources/{kind}", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.list", Handle: ListResource},
		{ID: "kubernetes.resource.get", Method: plugin.MethodGet, Path: "/resources/{kind}/get", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.get", Handle: GetResource},
		{ID: "kubernetes.resource.overview", Method: plugin.MethodGet, Path: "/resources/{kind}/overview", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.overview", Handle: ResourceOverview},
		{ID: "kubernetes.resource.columns", Method: plugin.MethodGet, Path: "/resources/{kind}/columns", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.columns", Handle: ColumnsForKind},
		{ID: "kubernetes.resource.watch", Method: plugin.MethodWS, Path: "/resources/{kind}/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.watch", Stream: WatchResource},
		{ID: "kubernetes.resource.object.watch", Method: plugin.MethodWS, Path: "/resources/{kind}/object/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.object.watch", Stream: WatchObject},

		{ID: "kubernetes.resource.delete", Method: plugin.MethodDelete, Path: "/resources/{kind}/delete", Permission: permDelete, Risk: plugin.RiskDestructive, AuditEvent: "kubernetes.resource.delete", Handle: DeleteResource},
		{ID: "kubernetes.resource.scale", Method: plugin.MethodPost, Path: "/resources/{kind}/scale", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.scale", Input: scaleSchema(), Handle: ScaleResource},
		{ID: "kubernetes.resource.restart", Method: plugin.MethodPost, Path: "/resources/{kind}/restart", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.restart", Handle: RestartResource},

		{ID: "kubernetes.node.cordon", Method: plugin.MethodPost, Path: "/nodes/cordon", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.node.cordon", Handle: CordonNode},
		{ID: "kubernetes.node.uncordon", Method: plugin.MethodPost, Path: "/nodes/uncordon", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.node.uncordon", Handle: UncordonNode},
		{ID: "kubernetes.node.drain", Method: plugin.MethodPost, Path: "/nodes/drain", Permission: permDelete, Risk: plugin.RiskDestructive, AuditEvent: "kubernetes.node.drain", Input: drainSchema(), Handle: DrainNode},

		{ID: "kubernetes.rollout.undo", Method: plugin.MethodPost, Path: "/resources/{kind}/rollout-undo", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.rollout.undo", Handle: RolloutUndo},
		{ID: "kubernetes.cronjob.trigger", Method: plugin.MethodPost, Path: "/resources/{kind}/trigger", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.cronjob.trigger", Handle: TriggerCronJob},

		{ID: "kubernetes.resource.yaml", Method: plugin.MethodGet, Path: "/resources/{kind}/yaml", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.yaml", Handle: GetYAML},
		{ID: "kubernetes.resource.yaml.watch", Method: plugin.MethodWS, Path: "/resources/{kind}/yaml/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.yaml.watch", Stream: WatchObjectYAML},
		{ID: "kubernetes.resource.template", Method: plugin.MethodGet, Path: "/resources/{kind}/template", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.template", Handle: TemplateYAML},
		{ID: "kubernetes.resource.apply", Method: plugin.MethodPost, Path: "/apply", Permission: permWrite, Risk: plugin.RiskWrite, AuditEvent: "kubernetes.resource.apply", Input: applySchema(), Handle: ApplyYAML},
		{ID: "kubernetes.resource.events", Method: plugin.MethodGet, Path: "/resources/{kind}/events", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.events", Handle: ResourceEvents},
		{ID: "kubernetes.resource.events.watch", Method: plugin.MethodWS, Path: "/resources/{kind}/events/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.resource.events.watch", Stream: WatchEvents},

		{ID: "kubernetes.pod.containers", Method: plugin.MethodGet, Path: "/pods/containers", Permission: "kubernetes.pods.logs", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.containers", Handle: PodContainers},
		{ID: "kubernetes.pod.logs", Method: plugin.MethodWS, Path: "/pods/logs", Permission: "kubernetes.pods.logs", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.logs", Stream: LogsStream},
		{ID: "kubernetes.workload.logs", Method: plugin.MethodWS, Path: "/resources/{kind}/logs", Permission: "kubernetes.pods.logs", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.workload.logs", Stream: WorkloadLogsStream},
		{ID: "kubernetes.pod.exec", Method: plugin.MethodWS, Path: "/pods/exec", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.exec", Stream: ExecStream},
		{ID: "kubernetes.pod.debug.create", Method: plugin.MethodPost, Path: "/pods/debug", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.debug.create", Input: debugSchema(), Handle: DebugCreate},

		{ID: "kubernetes.pod.files.list", Method: plugin.MethodGet, Path: "/pods/files/list/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.files.list", Handle: PodFilesList},
		{ID: "kubernetes.pod.files.read", Method: plugin.MethodGet, Path: "/pods/files/read/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.files.read", Handle: PodFileRead},
		{ID: "kubernetes.pod.files.download", Method: plugin.MethodGet, Path: "/pods/files/download/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.files.download", Handle: PodFileDownload},
		{ID: "kubernetes.pod.files.write", Method: plugin.MethodPut, Path: "/pods/files/write/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.files.write", Handle: PodFileWrite},
		{ID: "kubernetes.pod.files.upload", Method: plugin.MethodPost, Path: "/pods/files/upload/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.files.upload", Input: podUploadSchema(), Handle: PodFileUpload},
		{ID: "kubernetes.pod.files.mkdir", Method: plugin.MethodPost, Path: "/pods/files/mkdir/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.files.mkdir", Handle: PodFileMkdir},
		{ID: "kubernetes.pod.files.delete", Method: plugin.MethodDelete, Path: "/pods/files/delete/{path}", Permission: "kubernetes.pods.exec", Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.pod.files.delete", Handle: PodFileDelete},
		{ID: "kubernetes.cluster.shell", Method: plugin.MethodWS, Path: "/cluster/shell", Permission: permClusterShell, Risk: plugin.RiskPrivileged, AuditEvent: "kubernetes.cluster.shell", Stream: ClusterShellStream},

		{ID: "kubernetes.cluster.list", Method: plugin.MethodGet, Path: "/overview", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.cluster.list", Handle: ClusterList},
		{ID: "kubernetes.cluster.metrics", Method: plugin.MethodWS, Path: "/overview/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.cluster.metrics", Stream: ClusterMetrics},
		{ID: "kubernetes.node.metrics", Method: plugin.MethodWS, Path: "/nodes/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.node.metrics", Stream: NodeMetrics},
		{ID: "kubernetes.pod.metrics", Method: plugin.MethodWS, Path: "/pods/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.metrics", Stream: PodMetrics},
		{ID: "kubernetes.node.pods", Method: plugin.MethodGet, Path: "/nodes/pods", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.node.pods", Handle: NodePods},
		{ID: "kubernetes.workload.pods", Method: plugin.MethodGet, Path: "/resources/{kind}/pods", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.workload.pods", Handle: WorkloadPods},

		{ID: "kubernetes.tree.helm", Method: plugin.MethodGet, Path: "/tree/helm", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.tree.helm", Handle: TreeHelm},
		{ID: "kubernetes.helm.releases", Method: plugin.MethodGet, Path: "/helm/releases", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.helm.releases", Handle: HelmReleases},
		{ID: "kubernetes.helm.release", Method: plugin.MethodGet, Path: "/helm/release", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.helm.release", Handle: HelmRelease},

		{ID: "kubernetes.service.open", Method: plugin.MethodGet, Path: "/services/open", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.service.open", Input: openPortSchema("kubernetes.service.open.ports"), Handle: ServiceProxyURL},
		{ID: "kubernetes.service.open.ports", Method: plugin.MethodGet, Path: "/services/open/ports", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.service.open.ports", Handle: ServiceOpenPorts},
		{ID: "kubernetes.pod.open", Method: plugin.MethodGet, Path: "/pods/open", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.open", Input: openPortSchema("kubernetes.pod.open.ports"), Handle: PodProxyURL},
		{ID: "kubernetes.pod.open.ports", Method: plugin.MethodGet, Path: "/pods/open/ports", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "kubernetes.pod.open.ports", Handle: PodOpenPorts},
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

func drainSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Drain",
		Fields: []plugin.Field{
			{Key: "gracePeriodSeconds", Label: "Grace period (s)", Type: plugin.FieldStepper, Default: 30, Help: "Maximum termination grace period for evicted Pods.", Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}, {Type: plugin.ValidatorMax, Value: 3600}}},
			{Key: "force", Label: "Evict unmanaged Pods", Type: plugin.FieldToggle, Help: "Also evict Pods without a controller owner. DaemonSet and mirror Pods are still skipped."},
		},
	}}}
}

func scaleSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{
		Name: "Scale",
		Fields: []plugin.Field{
			{Key: "replicas", Label: "Replicas", Type: plugin.FieldStepper, Required: true, Help: "Set the desired replica count for this workload.", Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 0}}},
		},
	}}}
}
