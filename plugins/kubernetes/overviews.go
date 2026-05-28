package kubernetes

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng35/shellcn/internal/plugin"
)

const clusterKind = "cluster"

var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

func mapPods(items []unstructured.Unstructured) []Row {
	rows := make([]Row, 0, len(items))
	for i := range items {
		o := items[i].Object
		row := commonRow(o)
		for k, v := range podRow(o) {
			row[k] = v
		}
		row["ref"] = plugin.ResourceRef{Kind: "pod", Namespace: refNS(o), Name: refName(o), UID: str(o, "metadata", "uid")}
		rows = append(rows, row)
	}
	return rows
}

// ClusterList returns the single cluster row backing the Overview dashboard.
func ClusterList(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	row := Row{"name": "Cluster", "uid": clusterKind, "ref": plugin.ResourceRef{Kind: clusterKind, Name: "Cluster", UID: clusterKind}}
	if v, err := s.clientset.Discovery().ServerVersion(); err == nil {
		row["version"] = v.GitVersion
	}
	if nodes, err := s.clientset.CoreV1().Nodes().List(rc.Ctx, metav1.ListOptions{}); err == nil {
		row["nodes"] = len(nodes.Items)
	}
	return plugin.Page[Row]{Items: []Row{row}, Total: ptr(1)}, nil
}

// NodePods lists the pods scheduled on a node (for the node detail).
func NodePods(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	list, err := s.Dynamic().Resource(podsGVR).List(rc.Ctx, metav1.ListOptions{
		FieldSelector: nodePodSelector(rc.Param("name")),
	})
	if err != nil {
		return nil, apiErr(err)
	}
	return pageRows(rc, mapPods(list.Items))
}

// WorkloadPods lists the pods a workload owns, matched by its selector labels.
func WorkloadPods(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return nil, apiErr(err)
	}
	sel := labelSelector(mapField(o.Object, "spec", "selector", "matchLabels"))
	if sel == "" {
		return pageRows(rc, nil)
	}
	list, err := s.Dynamic().Resource(podsGVR).Namespace(o.GetNamespace()).List(rc.Ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return nil, apiErr(err)
	}
	return pageRows(rc, mapPods(list.Items))
}

func labelSelector(labels map[string]any) string {
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		if sv, ok := v.(string); ok {
			parts = append(parts, k+"="+sv)
		}
	}
	return strings.Join(parts, ",")
}

// clusterResourceType is the Overview dashboard: live cluster metrics, the node
// list, and recent events — composed from generic panels.
func clusterResourceType() plugin.ResourceType {
	dash := plugin.DashboardConfig{Cells: []plugin.DashboardCell{
		{
			Key: "metrics", Label: "Cluster metrics", Panel: plugin.PanelMetrics, Span: 2,
			Source: &plugin.DataSource{RouteID: "kubernetes.cluster.metrics", Method: plugin.MethodWS}, Config: clusterMetricsConfig(),
		},
		{
			Key: "nodes", Label: "Nodes", Panel: plugin.PanelTable, Span: 2,
			Source: &plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": "node"}},
			Config: kindColumnsConfig("node"),
		},
		{
			Key: "events", Label: "Recent events", Panel: plugin.PanelTable, Span: 2,
			Source: &plugin.DataSource{RouteID: "kubernetes.resource.list", Params: map[string]string{"kind": "event"}},
			Config: kindColumnsConfig("event"),
		},
	}}
	return plugin.ResourceType{
		Kind:    clusterKind,
		Title:   "Overview",
		List:    plugin.DataSource{RouteID: "kubernetes.cluster.list"},
		Columns: []plugin.Column{nameCol(), col("version", "Version"), col("nodes", "Nodes", num)},
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "Cluster Overview"},
			Tabs: []plugin.Tab{
				{Key: "dashboard", Label: "Overview", Icon: lucide("layout-dashboard"), Panel: plugin.PanelDashboard, Config: dash.Map()},
			},
		},
	}
}

// kindColumnsConfig reuses a catalog kind's declared columns for an embedded
// dashboard table, so it matches the full list view (badges and all).
func kindColumnsConfig(name string) map[string]any {
	if k, ok := kindByName(name); ok {
		return plugin.TableConfig{Columns: k.columns}.Map()
	}
	return nil
}

func podsTableConfig() map[string]any {
	return plugin.TableConfig{Columns: []plugin.Column{
		col("name", "Name"), col("ready", "Ready", notSort), col("status", "Status", statusBadge(podSeverities)),
		col("restarts", "Restarts", num), col("node", "Node"), ageCol(),
	}}.Map()
}

// nodeDetailTabs adds live Metrics + scheduled Pods to a node's detail.
func nodeDetailTabs() []plugin.Tab {
	return []plugin.Tab{
		{
			Key: "metrics", Label: "Metrics", Icon: lucide("activity"), Panel: plugin.PanelMetrics,
			Source: &plugin.DataSource{RouteID: "kubernetes.node.metrics", Method: plugin.MethodWS, Params: map[string]string{"name": "${resource.name}"}},
			Config: nodeMetricsConfig(),
		},
		{
			Key: "pods", Label: "Pods", Icon: lucide("box"), Panel: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "kubernetes.node.pods", Params: map[string]string{"name": "${resource.name}"}},
			Config: podsTableConfig(),
		},
	}
}

// workloadPodsTab adds the owned-Pods table to a workload's detail.
func workloadPodsTab(kindName string) plugin.Tab {
	return plugin.Tab{
		Key: "pods", Label: "Pods", Icon: lucide("box"), Panel: plugin.PanelTable,
		Source: &plugin.DataSource{RouteID: "kubernetes.workload.pods", Params: map[string]string{"kind": kindName, "namespace": "${resource.namespace}", "name": "${resource.name}"}},
		Config: podsTableConfig(),
	}
}

func clusterMetricsConfig() map[string]any {
	return plugin.MetricsConfig{
		Gauges: []plugin.MetricGauge{
			{Key: "cpuPct", Label: "CPU", Unit: "%", Max: 100},
			{Key: "memPct", Label: "Memory", Unit: "%", Max: 100},
		},
		Stats: []plugin.MetricStat{
			{Key: "pods", Label: "Pods"},
			{Key: "nodes", Label: "Nodes"},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU cores"},
			{Key: "mem", Label: "Memory", Unit: "bytes"},
		},
		History: 60,
	}.Map()
}

func nodeMetricsConfig() map[string]any {
	return plugin.MetricsConfig{
		Gauges: []plugin.MetricGauge{
			{Key: "cpuPct", Label: "CPU", Unit: "%", Max: 100},
			{Key: "memPct", Label: "Memory", Unit: "%", Max: 100},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU cores"},
			{Key: "mem", Label: "Memory", Unit: "bytes"},
		},
		History: 60,
	}.Map()
}
