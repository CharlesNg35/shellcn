package kubernetes

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// category is an expandable sidebar group that drills into its kinds, mirroring
// the Lens cluster menu. Single-kind, top-level entries (Nodes/Namespaces/Events)
// are rendered separately by tree().
type category struct {
	key, label, icon string
}

var categories = []category{
	{"workloads", "Workloads", "layers"},
	{"config", "Config", "sliders-horizontal"},
	{"network", "Network", "globe"},
	{"storage", "Storage", "hard-drive"},
	{"access", "Access Control", "shield"},
}

// kind is one managed resource type. The catalog is the single source that
// drives the manifest tree, resource types, and the generic list/get/watch/
// delete routes — adding a kind is one entry, no new routes or frontend.
type kind struct {
	name       string // Ref/route key, e.g. "pod"
	title      string
	category   string
	icon       string
	gvr        schema.GroupVersionResource
	namespaced bool
	redact     bool // never return object data (Secrets)
	columns    []plugin.Column
	extra      func(obj) Row // cells beyond commonRow
	actionIDs  []string      // row + detail actions (Edit/Create added generically)
	detailTabs []plugin.Tab  // extra detail tabs beyond Overview/YAML/Events
	subgroup   string        // optional nested sub-group within the category
}

// subgroupLabels names the nested sub-groups a category can expand into.
var subgroupLabels = map[string]string{"admissionpolicies": "Admission Policies"}

func col(key, label string, opts ...func(*plugin.Column)) plugin.Column {
	c := plugin.Column{Key: key, Label: label, Sortable: true}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func badge(c *plugin.Column)   { c.Type = plugin.ColumnBadge }
func num(c *plugin.Column)     { c.Type = plugin.ColumnNumber }
func notSort(c *plugin.Column) { c.Sortable = false }

func nameCol() plugin.Column { return col("name", "Name") }
func nsCol() plugin.Column   { return col("namespace", "Namespace") }
func ageCol() plugin.Column  { return col("age", "Age") }

// Action sets referenced by kinds. Edit/logs/exec are added by steps 3–4.
var (
	scalable   = []string{"kubernetes.resource.scale", "kubernetes.resource.restart", "kubernetes.resource.delete"}
	justDelete = []string{"kubernetes.resource.delete"}
)

// kinds is the built-in resource catalog (CRDs are discovered at runtime).
var kinds = []kind{
	{
		name: "pod", title: "Pods", category: "workloads", icon: "box", namespaced: true,
		gvr:        schema.GroupVersionResource{Version: "v1", Resource: "pods"},
		columns:    []plugin.Column{nameCol(), nsCol(), col("ready", "Ready", notSort), col("status", "Status", badge), col("restarts", "Restarts", num), col("node", "Node"), col("podIP", "IP", notSort), ageCol()},
		extra:      podRow,
		actionIDs:  []string{"kubernetes.pod.open", "kubernetes.pod.logs", "kubernetes.pod.exec", "kubernetes.resource.delete"},
		detailTabs: podDetailTabs(),
	},
	{
		name: "deployment", title: "Deployments", category: "workloads", icon: "layers", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		columns: []plugin.Column{nameCol(), nsCol(), col("ready", "Ready", notSort), col("upToDate", "Up-to-date", num), col("available", "Available", num), ageCol()},
		extra:   deploymentRow, actionIDs: scalable, detailTabs: []plugin.Tab{workloadPodsTab("deployment")},
	},
	{
		name: "statefulset", title: "StatefulSets", category: "workloads", icon: "layers", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		columns: []plugin.Column{nameCol(), nsCol(), col("ready", "Ready", notSort), ageCol()},
		extra:   statefulSetRow, actionIDs: scalable, detailTabs: []plugin.Tab{workloadPodsTab("statefulset")},
	},
	{
		name: "daemonset", title: "DaemonSets", category: "workloads", icon: "layers", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		columns: []plugin.Column{nameCol(), nsCol(), col("desired", "Desired", num), col("ready", "Ready", num), col("available", "Available", num), ageCol()},
		extra:   daemonSetRow, actionIDs: []string{"kubernetes.resource.restart", "kubernetes.resource.delete"}, detailTabs: []plugin.Tab{workloadPodsTab("daemonset")},
	},
	{
		name: "replicaset", title: "ReplicaSets", category: "workloads", icon: "layers", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		columns: []plugin.Column{nameCol(), nsCol(), col("desired", "Desired", num), col("current", "Current", num), col("ready", "Ready", num), ageCol()},
		extra:   replicaSetRow, actionIDs: scalable, detailTabs: []plugin.Tab{workloadPodsTab("replicaset")},
	},
	{
		name: "job", title: "Jobs", category: "workloads", icon: "square-check", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
		columns: []plugin.Column{nameCol(), nsCol(), col("completions", "Completions", notSort), col("active", "Active", num), ageCol()},
		extra:   jobRow, actionIDs: justDelete,
	},
	{
		name: "cronjob", title: "CronJobs", category: "workloads", icon: "clock", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
		columns: []plugin.Column{nameCol(), nsCol(), col("schedule", "Schedule"), col("suspend", "Suspend"), col("active", "Active", num), col("lastSchedule", "Last schedule", func(c *plugin.Column) { c.Type = plugin.ColumnDateTime }), ageCol()},
		extra:   cronJobRow, actionIDs: justDelete,
	},
	{
		name: "replicationcontroller", title: "Replication Controllers", category: "workloads", icon: "layers", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "replicationcontrollers"},
		columns: []plugin.Column{nameCol(), nsCol(), col("desired", "Desired", num), col("current", "Current", num), col("ready", "Ready", num), ageCol()},
		extra:   replicationControllerRow, actionIDs: scalable,
	},
	{
		name: "service", title: "Services", category: "network", icon: "network", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "services"},
		columns: []plugin.Column{nameCol(), nsCol(), col("type", "Type", badge), col("clusterIP", "Cluster IP", notSort), col("ports", "Ports", notSort), ageCol()},
		extra:   serviceRow, actionIDs: []string{"kubernetes.service.open", "kubernetes.resource.delete"},
	},
	{
		name: "endpoints", title: "Endpoints", category: "network", icon: "network", namespaced: true,
		gvr:       schema.GroupVersionResource{Version: "v1", Resource: "endpoints"},
		columns:   []plugin.Column{nameCol(), nsCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "endpointslice", title: "Endpoint Slices", category: "network", icon: "network", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"},
		columns: []plugin.Column{nameCol(), nsCol(), col("addressType", "Address type"), col("endpoints", "Endpoints", num), col("ports", "Ports", num), ageCol()},
		extra:   endpointSliceRow, actionIDs: justDelete,
	},
	{
		name: "ingressclass", title: "Ingress Classes", category: "network", icon: "globe",
		gvr:     schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses"},
		columns: []plugin.Column{nameCol(), col("controller", "Controller"), ageCol()},
		extra:   ingressClassRow, actionIDs: justDelete,
	},
	{
		name: "ingress", title: "Ingresses", category: "network", icon: "globe", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		columns: []plugin.Column{nameCol(), nsCol(), col("class", "Class"), col("hosts", "Hosts", notSort), ageCol()},
		extra:   ingressRow, actionIDs: justDelete,
	},
	{
		name: "networkpolicy", title: "Network Policies", category: "network", icon: "shield-check", namespaced: true,
		gvr:       schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		columns:   []plugin.Column{nameCol(), nsCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "persistentvolumeclaim", title: "Persistent Volume Claims", category: "storage", icon: "hard-drive", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"},
		columns: []plugin.Column{nameCol(), nsCol(), col("status", "Status", badge), col("volume", "Volume"), col("capacity", "Capacity", notSort), col("storageClass", "Storage class"), ageCol()},
		extra:   pvcRow, actionIDs: justDelete,
	},
	{
		name: "persistentvolume", title: "Persistent Volumes", category: "storage", icon: "hard-drive",
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"},
		columns: []plugin.Column{nameCol(), col("capacity", "Capacity", notSort), col("status", "Status", badge), col("claim", "Claim"), col("storageClass", "Storage class"), col("reclaim", "Reclaim"), ageCol()},
		extra:   pvRow, actionIDs: justDelete,
	},
	{
		name: "storageclass", title: "Storage Classes", category: "storage", icon: "database",
		gvr:     schema.GroupVersionResource{Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
		columns: []plugin.Column{nameCol(), col("provisioner", "Provisioner"), col("reclaim", "Reclaim"), col("bindingMode", "Binding mode"), col("allowExpand", "Expandable"), ageCol()},
		extra:   storageClassRow, actionIDs: justDelete,
	},
	{
		name: "configmap", title: "Config Maps", category: "config", icon: "file-text", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		columns: []plugin.Column{nameCol(), nsCol(), col("keys", "Keys", num), ageCol()},
		extra:   configMapRow, actionIDs: justDelete,
	},
	{
		name: "secret", title: "Secrets", category: "config", icon: "key-round", namespaced: true, redact: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
		columns: []plugin.Column{nameCol(), nsCol(), col("type", "Type"), col("keys", "Keys", num), ageCol()},
		extra:   secretRow, actionIDs: justDelete,
	},
	{
		name: "resourcequota", title: "Resource Quotas", category: "config", icon: "gauge", namespaced: true,
		gvr:       schema.GroupVersionResource{Version: "v1", Resource: "resourcequotas"},
		columns:   []plugin.Column{nameCol(), nsCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "limitrange", title: "Limit Ranges", category: "config", icon: "gauge", namespaced: true,
		gvr:       schema.GroupVersionResource{Version: "v1", Resource: "limitranges"},
		columns:   []plugin.Column{nameCol(), nsCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "horizontalpodautoscaler", title: "Horizontal Pod Autoscalers", category: "config", icon: "trending-up", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		columns: []plugin.Column{nameCol(), nsCol(), col("reference", "Reference"), col("minPods", "Min", num), col("maxPods", "Max", num), col("replicas", "Replicas", num), ageCol()},
		extra:   hpaRow, actionIDs: justDelete,
	},
	{
		name: "poddisruptionbudget", title: "Pod Disruption Budgets", category: "config", icon: "shield-alert", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
		columns: []plugin.Column{nameCol(), nsCol(), col("minAvailable", "Min available"), col("maxUnavailable", "Max unavailable"), col("currentHealthy", "Healthy", num), ageCol()},
		extra:   pdbRow, actionIDs: justDelete,
	},
	{
		name: "priorityclass", title: "Priority Classes", category: "config", icon: "arrow-up-narrow-wide",
		gvr:     schema.GroupVersionResource{Group: "scheduling.k8s.io", Version: "v1", Resource: "priorityclasses"},
		columns: []plugin.Column{nameCol(), col("value", "Value", num), col("globalDefault", "Global default"), ageCol()},
		extra:   priorityClassRow, actionIDs: justDelete,
	},
	{
		name: "runtimeclass", title: "Runtime Classes", category: "config", icon: "cpu",
		gvr:     schema.GroupVersionResource{Group: "node.k8s.io", Version: "v1", Resource: "runtimeclasses"},
		columns: []plugin.Column{nameCol(), col("handler", "Handler"), ageCol()},
		extra:   runtimeClassRow, actionIDs: justDelete,
	},
	{
		name: "lease", title: "Leases", category: "config", icon: "timer", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "coordination.k8s.io", Version: "v1", Resource: "leases"},
		columns: []plugin.Column{nameCol(), nsCol(), col("holder", "Holder"), ageCol()},
		extra:   leaseRow, actionIDs: justDelete,
	},
	{
		name: "mutatingwebhookconfiguration", title: "Mutating Webhook Configs", category: "config", icon: "webhook",
		gvr:     schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations"},
		columns: []plugin.Column{nameCol(), col("webhooks", "Webhooks", num), ageCol()},
		extra:   webhookConfigRow, actionIDs: justDelete,
	},
	{
		name: "validatingwebhookconfiguration", title: "Validating Webhook Configs", category: "config", icon: "webhook",
		gvr:     schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations"},
		columns: []plugin.Column{nameCol(), col("webhooks", "Webhooks", num), ageCol()},
		extra:   webhookConfigRow, actionIDs: justDelete,
	},
	{
		name: "validatingadmissionpolicy", title: "Validating Admission Policies", category: "config", icon: "shield-check",
		gvr:       schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicies"},
		columns:   []plugin.Column{nameCol(), ageCol()},
		actionIDs: justDelete, subgroup: "admissionpolicies",
	},
	{
		name: "validatingadmissionpolicybinding", title: "Validating Admission Policy Bindings", category: "config", icon: "shield-check",
		gvr:       schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicybindings"},
		columns:   []plugin.Column{nameCol(), ageCol()},
		actionIDs: justDelete, subgroup: "admissionpolicies",
	},
	{
		name: "serviceaccount", title: "Service Accounts", category: "access", icon: "user", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		columns: []plugin.Column{nameCol(), nsCol(), col("secrets", "Secrets", num), ageCol()},
		extra:   serviceAccountRow, actionIDs: justDelete,
	},
	{
		name: "role", title: "Roles", category: "access", icon: "shield", namespaced: true,
		gvr:       schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		columns:   []plugin.Column{nameCol(), nsCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "rolebinding", title: "Role Bindings", category: "access", icon: "shield", namespaced: true,
		gvr:     schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		columns: []plugin.Column{nameCol(), nsCol(), col("role", "Role"), col("subjects", "Subjects", notSort), ageCol()},
		extra:   roleBindingRow, actionIDs: justDelete,
	},
	{
		name: "clusterrole", title: "Cluster Roles", category: "access", icon: "shield-half",
		gvr:       schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		columns:   []plugin.Column{nameCol(), ageCol()},
		actionIDs: justDelete,
	},
	{
		name: "clusterrolebinding", title: "Cluster Role Bindings", category: "access", icon: "shield-half",
		gvr:     schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		columns: []plugin.Column{nameCol(), col("role", "Role"), col("subjects", "Subjects", notSort), ageCol()},
		extra:   roleBindingRow, actionIDs: justDelete,
	},
	{
		name: "namespace", title: "Namespaces", category: "cluster", icon: "box",
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
		columns: []plugin.Column{nameCol(), col("status", "Status", badge), ageCol()},
		extra:   namespaceExtra, actionIDs: justDelete,
	},
	{
		name: "node", title: "Nodes", category: "cluster", icon: "server",
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "nodes"},
		columns: []plugin.Column{nameCol(), col("status", "Status", badge), col("roles", "Roles"), col("version", "Version"), ageCol()},
		extra:   nodeRow, actionIDs: []string{"kubernetes.node.cordon", "kubernetes.node.uncordon"}, detailTabs: nodeDetailTabs(),
	},
	{
		name: "event", title: "Events", category: "cluster", icon: "bell", namespaced: true,
		gvr:     schema.GroupVersionResource{Version: "v1", Resource: "events"},
		columns: []plugin.Column{col("type", "Type", badge), col("reason", "Reason"), col("object", "Object"), col("message", "Message", notSort), col("count", "Count", num), ageCol()},
		extra:   eventRow,
	},
	{
		// "Definitions" under Custom Resources: the CRDs themselves.
		name: "customresourcedefinition", title: "Definitions", category: "custom", icon: "puzzle",
		gvr:     schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"},
		columns: []plugin.Column{nameCol(), col("group", "Group"), col("kind", "Kind"), col("scope", "Scope"), ageCol()},
		extra:   crdDefRow, actionIDs: justDelete,
	},
}

func kindByName(name string) (kind, bool) {
	for _, k := range kinds {
		if k.name == name {
			return k, true
		}
	}
	return kind{}, false
}
