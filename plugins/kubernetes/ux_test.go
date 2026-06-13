package kubernetes

import (
	"context"
	"errors"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// resourceActionIDs collects every action a resource list/row/detail references.
func resourceActionIDs() map[string]bool {
	ids := map[string]bool{}
	add := func(list []string) {
		for _, id := range list {
			ids[id] = true
		}
	}
	for _, r := range resources() {
		add(r.Actions.Detail)
		add(r.Actions.Row)
		add(r.Actions.Toolbar)
	}
	return ids
}

// Dock is reserved for header affordances; a resource-bound action must surface
// as a detail tab or a dialog, never a dock.
func TestResourceActionsNeverOpenDock(t *testing.T) {
	referenced := resourceActionIDs()
	for _, a := range actions() {
		if referenced[a.ID] && a.Open == plugin.OpenDock {
			t.Errorf("resource action %q opens a dock; use a detail tab or a dialog", a.ID)
		}
	}
}

func TestHeaderActionsResolveToActions(t *testing.T) {
	byID := map[string]plugin.Action{}
	for _, a := range actions() {
		byID[a.ID] = a
	}
	for _, id := range headerActionIDs() {
		if _, ok := byID[id]; !ok {
			t.Errorf("header action %q has no matching action", id)
		}
	}
	if a := byID["kubernetes.cluster.shell"]; a.Open != plugin.OpenDock || a.Panel != plugin.PanelTerminal {
		t.Errorf("cluster shell should dock a terminal, got open=%q panel=%q", a.Open, a.Panel)
	}
	if a := byID["kubernetes.cluster.apply"]; a.Open != plugin.OpenDialog || a.Panel != plugin.PanelCodeEditor {
		t.Errorf("apply YAML should open a code-editor dialog, got open=%q panel=%q", a.Open, a.Panel)
	}
}

func TestClusterShellUsesDedicatedPermission(t *testing.T) {
	for _, r := range Routes() {
		if r.ID == "kubernetes.cluster.shell" {
			if r.Permission != permClusterShell {
				t.Fatalf("cluster shell permission = %q, want %q", r.Permission, permClusterShell)
			}
			if r.Permission == "kubernetes.pods.exec" {
				t.Fatal("cluster shell must not share the pod exec permission")
			}
			return
		}
	}
	t.Fatal("cluster shell route missing")
}

func TestEventsUseTimelinePanels(t *testing.T) {
	cluster := clusterResourceType()
	dashboard := cluster.Detail.Tabs[0]
	cfg, ok := dashboard.Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("cluster dashboard config = %T, want DashboardConfig", dashboard.Config)
	}
	found := false
	for _, cell := range cfg.Cells {
		if cell.Key != "events" {
			continue
		}
		found = true
		if cell.Type != plugin.PanelTimeline {
			t.Fatalf("cluster events panel = %q, want timeline", cell.Type)
		}
		if timeline, ok := cell.Config.(plugin.TimelineConfig); !ok || timeline.RefreshIntervalMs == 0 {
			t.Fatalf("cluster events timeline config = %#v, want refreshable TimelineConfig", cell.Config)
		}
	}
	if !found {
		t.Fatal("cluster dashboard events panel missing")
	}

	var pod kind
	for _, k := range kinds {
		if k.name == "pod" {
			pod = k
			break
		}
	}
	if pod.name == "" {
		t.Fatal("pod kind missing")
	}
	podEvents := eventsTab(pod)
	if podEvents.Type != plugin.PanelTimeline {
		t.Fatalf("pod events panel = %q, want timeline", podEvents.Type)
	}
	timeline, ok := podEvents.Config.(plugin.TimelineConfig)
	if !ok || timeline.TimestampField != "createdAt" || timeline.SeverityField != "type" {
		t.Fatalf("pod events timeline config = %#v", podEvents.Config)
	}
}

func TestResourceOverviewsUseStructuredPanels(t *testing.T) {
	for _, k := range kinds {
		res := resourceType(k)
		if len(res.Detail.Tabs) == 0 {
			t.Fatalf("%s detail tabs missing", k.name)
		}
		overview := res.Detail.Tabs[0]
		if overview.Key != "overview" || overview.Type != plugin.PanelObjectDetail {
			t.Fatalf("%s overview = key %q panel %q, want structured object detail", k.name, overview.Key, overview.Type)
		}
		if overview.Source == nil || overview.Source.RouteID != "kubernetes.resource.overview" {
			t.Fatalf("%s overview source = %+v", k.name, overview.Source)
		}
		cfg, ok := overview.Config.(plugin.ObjectDetailConfig)
		if !ok || len(cfg.Sections) == 0 || !cfg.RawToggle {
			t.Fatalf("%s overview config = %#v, want structured sections with raw toggle", k.name, overview.Config)
		}
	}
}

func TestPodDetailHasMetricsLogsAndShell(t *testing.T) {
	var pod kind
	for _, k := range kinds {
		if k.name == "pod" {
			pod = k
		}
	}
	if pod.name == "" {
		t.Fatal("pod kind missing")
	}
	res := resourceType(pod)
	var order []string
	for _, tab := range res.Detail.Tabs {
		order = append(order, tab.Key)
	}
	want := []string{"overview", "yaml", "metrics", "logs", "terminal", "events"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("pod detail tabs = %v, want %v", order, want)
	}
	metrics := res.Detail.Tabs[2]
	if metrics.Type != plugin.PanelMetrics || metrics.Source == nil || metrics.Source.RouteID != "kubernetes.pod.metrics" {
		t.Fatalf("pod metrics tab = %+v", metrics)
	}
	cfg, ok := metrics.Config.(plugin.MetricsConfig)
	if !ok || len(cfg.Stats) == 0 || len(cfg.Usage) == 0 || len(cfg.Series) == 0 || len(cfg.Gauges) != 0 {
		t.Fatalf("pod metrics config = %#v", metrics.Config)
	}
	if !conditionRequiresStatus(metrics.VisibleWhen, "Running") {
		t.Fatalf("pod metrics should only show for running pods, got %#v", metrics.VisibleWhen)
	}
	terminal := res.Detail.Tabs[4]
	if !conditionRequiresStatus(terminal.VisibleWhen, "Running") {
		t.Fatalf("pod shell should only show for running pods, got %#v", terminal.VisibleWhen)
	}
	logs := res.Detail.Tabs[3]
	if logs.VisibleWhen != nil {
		t.Fatalf("pod logs should stay available for terminated/crashing pods, got %#v", logs.VisibleWhen)
	}
}

func TestPodOpenRequiresRunningPodWithPorts(t *testing.T) {
	for _, a := range actions() {
		if a.ID != "kubernetes.pod.open" {
			continue
		}
		if a.EnabledWhen == nil || len(a.EnabledWhen.AllOf) != 2 {
			t.Fatalf("pod open should require ports and running status, got %#v", a.EnabledWhen)
		}
		if !conditionHasRule(a.EnabledWhen, "ports", plugin.OpNotEmpty, nil) {
			t.Fatalf("pod open should require exposed ports, got %#v", a.EnabledWhen)
		}
		if !conditionRequiresStatus(a.EnabledWhen, "Running") {
			t.Fatalf("pod open should require a running pod, got %#v", a.EnabledWhen)
		}
		return
	}
	t.Fatal("pod open action missing")
}

func TestServiceOpenRequiresForwardablePorts(t *testing.T) {
	a := actionByID("kubernetes.service.open")
	if a.ID == "" {
		t.Fatal("service open action missing")
	}
	if !conditionHasRule(a.EnabledWhen, "ports", plugin.OpNotEmpty, nil) {
		t.Fatalf("service open should require ports, got %#v", a.EnabledWhen)
	}
	if !conditionHasRule(a.EnabledWhen, "type", plugin.OpNeq, "ExternalName") {
		t.Fatalf("service open should be disabled for ExternalName services, got %#v", a.EnabledWhen)
	}
}

func TestControllerOwnedKindsDoNotExposeCreateOrDelete(t *testing.T) {
	for _, name := range []string{"event", "endpoints", "endpointslice", "lease", "node"} {
		k, ok := kindByName(name)
		if !ok {
			t.Fatalf("kind %q missing", name)
		}
		res := resourceType(k)
		if hasAction(res.Actions.Toolbar, "kubernetes.create."+name) {
			t.Errorf("%s should not expose Create", name)
		}
		if hasAction(res.Actions.Row, "kubernetes.resource.delete") || hasAction(res.Actions.Detail, "kubernetes.resource.delete") {
			t.Errorf("%s should not expose generic Delete, got row=%v detail=%v", name, res.Actions.Row, res.Actions.Detail)
		}
	}
}

func TestWorkloadStatusIsVisibleInListsAndHeaders(t *testing.T) {
	for _, name := range []string{"deployment", "statefulset", "daemonset", "replicaset", "job"} {
		k, ok := kindByName(name)
		if !ok {
			t.Fatalf("kind %q missing", name)
		}
		res := resourceType(k)
		if !hasColumn(res.Columns, "status") {
			t.Errorf("%s should expose a status badge column", name)
		}
		if res.Detail.Header.StatusField != "status" {
			t.Errorf("%s detail header status = %q, want status", name, res.Detail.Header.StatusField)
		}
	}
}

func TestScaleSchemaDoesNotDefaultToOneReplica(t *testing.T) {
	s := scaleSchema()
	field := s.Groups[0].Fields[0]
	if field.Key != "replicas" || !field.Required {
		t.Fatalf("scale replicas field = %+v", field)
	}
	if field.Default != nil {
		t.Fatalf("scale replicas should not default to a destructive value, got %v", field.Default)
	}
	if len(field.Validators) == 0 || field.Validators[0].Type != plugin.ValidatorMin || field.Validators[0].Value != 0 {
		t.Fatalf("scale replicas should validate a non-negative count, got %+v", field.Validators)
	}
}

func actionByID(id string) plugin.Action {
	for _, a := range actions() {
		if a.ID == id {
			return a
		}
	}
	return plugin.Action{}
}

func hasAction(actions []string, id string) bool {
	for _, got := range actions {
		if got == id {
			return true
		}
	}
	return false
}

func hasColumn(columns []plugin.Column, key string) bool {
	for _, got := range columns {
		if got.Key == key {
			return true
		}
	}
	return false
}

func conditionRequiresStatus(c *plugin.Condition, status string) bool {
	return conditionHasRule(c, "status", plugin.OpEq, status)
}

func conditionHasRule(c *plugin.Condition, field string, op plugin.Operator, value any) bool {
	if c == nil {
		return false
	}
	for _, r := range append(c.AllOf, c.AnyOf...) {
		if r.Field == field && r.Op == op {
			if value == nil {
				return true
			}
			return r.Value == value
		}
	}
	return false
}

func TestCustomResourceDetailHasYAMLAndEvents(t *testing.T) {
	res := customResourceType()
	if strings.Join(res.Actions.Row, ",") != "kubernetes.customresource.delete" || strings.Join(res.Actions.Detail, ",") != "kubernetes.customresource.delete" {
		t.Fatalf("custom resource should use scope-aware delete actions, got row=%v detail=%v", res.Actions.Row, res.Actions.Detail)
	}
	var keys []string
	for _, tab := range res.Detail.Tabs {
		keys = append(keys, tab.Key)
		switch tab.Key {
		case "yaml":
			if tab.Type != plugin.PanelCodeEditor || tab.Source == nil || tab.Source.Params["kind"] != "${resource.scope}" {
				t.Fatalf("custom resource YAML tab should edit the concrete CRD kind, got %+v", tab)
			}
		case "events":
			if tab.Type != plugin.PanelTimeline || tab.Source == nil || tab.Source.Params["kind"] != "${resource.scope}" {
				t.Fatalf("custom resource events tab should use the concrete CRD kind, got %+v", tab)
			}
			if _, ok := tab.Config.(plugin.TimelineConfig); !ok {
				t.Fatalf("custom resource events config = %T, want TimelineConfig", tab.Config)
			}
		}
	}
	want := []string{"overview", "yaml", "events"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("custom resource detail tabs = %v, want %v", keys, want)
	}
}

func TestCustomResourceDeleteUsesConcreteScopeKind(t *testing.T) {
	for _, a := range actions() {
		if a.ID != "kubernetes.customresource.delete" {
			continue
		}
		if a.RouteID != "kubernetes.resource.delete" {
			t.Fatalf("custom resource delete route = %q", a.RouteID)
		}
		if a.Params["kind"] != "${resource.scope}" {
			t.Fatalf("custom resource delete kind param = %q, want concrete scope", a.Params["kind"])
		}
		if !a.Confirm || a.OnSuccess == nil || a.OnSuccess.Navigate != plugin.NavigateList {
			t.Fatalf("custom resource delete should confirm and navigate to list, got %+v", a)
		}
		return
	}
	t.Fatal("custom resource delete action missing")
}

func TestAuditShellRBACUsesStreamAuditHook(t *testing.T) {
	var gotResult plugin.AuditResult
	var gotParams map[string]string
	var gotErr error
	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, nil, nil, nil, nil).
		WithAuditHook(func(_ context.Context, result plugin.AuditResult, params map[string]string, err error) {
			gotResult = result
			gotParams = params
			gotErr = err
		})
	err := errors.New("rbac denied")

	auditShellRBAC(rc, err)

	if gotResult != plugin.AuditError || !errors.Is(gotErr, err) {
		t.Fatalf("audit result = %q err=%v, want error %v", gotResult, gotErr, err)
	}
	if gotParams["operation"] != "cluster-shell-rbac" || gotParams["clusterRole"] != "cluster-admin" {
		t.Fatalf("audit params = %+v", gotParams)
	}
}

func TestInteractiveShellCommand(t *testing.T) {
	got := interactiveShellCommand(rc(nil, nil), true)
	if len(got) != 3 || got[0] != "/bin/sh" || got[1] != "-c" || !strings.Contains(got[2], "exec bash") {
		t.Errorf("a TTY session should prefer bash, got %v", got)
	}
	if got := interactiveShellCommand(rc(nil, nil), false); len(got) != 1 || got[0] != "/bin/sh" {
		t.Errorf("a non-TTY session should get a plain shell, got %v", got)
	}
	if got := interactiveShellCommand(rc(nil, map[string]string{"command": "/bin/zsh"}), true); len(got) != 1 || got[0] != "/bin/zsh" {
		t.Errorf("an explicit command should override, got %v", got)
	}
	candidates := interactiveShellCommands(rc(nil, nil), true)
	if len(candidates) < 2 || candidates[0][0] != "/bin/sh" || candidates[1][0] != "/bin/bash" {
		t.Errorf("a default TTY shell should include fallback candidates, got %v", candidates)
	}
	if got := interactiveShellCommands(rc(nil, map[string]string{"command": "/bin/zsh"}), true); len(got) != 1 || got[0][0] != "/bin/zsh" {
		t.Errorf("an explicit command should not fallback, got %v", got)
	}
}

func TestShellPodIsFixedAndReusable(t *testing.T) {
	p := shellPod(true)
	if p.Name != shellPodName {
		t.Errorf("shell pod needs a fixed name for reuse, got %q", p.Name)
	}
	if p.Labels[shellPodLabel] != "true" {
		t.Error("shell pod should be labelled as a managed cluster shell")
	}
	if p.Spec.ServiceAccountName != shellSAName {
		t.Errorf("shell pod should run as the dedicated SA, got %q", p.Spec.ServiceAccountName)
	}
	if shellPod(false).Spec.ServiceAccountName != "" {
		t.Error("without a usable SA the shell pod should fall back to the namespace default")
	}
}

func TestShellRBACGrantsClusterAdmin(t *testing.T) {
	crb := shellClusterRoleBinding()
	if crb.RoleRef.Name != "cluster-admin" {
		t.Errorf("shell binding should grant cluster-admin, got %q", crb.RoleRef.Name)
	}
	if len(crb.Subjects) != 1 || crb.Subjects[0].Name != shellSAName || crb.Subjects[0].Namespace != shellNamespace {
		t.Errorf("shell binding should target the shell SA, got %+v", crb.Subjects)
	}
}

func TestDeleteActionNavigatesToList(t *testing.T) {
	var del plugin.Action
	for _, a := range actions() {
		if a.ID == "kubernetes.resource.delete" {
			del = a
		}
	}
	if del.ID == "" {
		t.Fatal("delete action missing")
	}
	if del.OnSuccess == nil || del.OnSuccess.Navigate != plugin.NavigateList {
		t.Errorf("delete should navigate back to the list, got %+v", del.OnSuccess)
	}
}

func TestNamespaceIsAGlobalScope(t *testing.T) {
	scope := namespaceScope()
	if scope.Param != "namespace" {
		t.Errorf("namespace scope should set the namespace param, got %q", scope.Param)
	}
	if scope.Control != plugin.ScopeSelect {
		t.Errorf("namespace scope should explicitly render as a select, got %q", scope.Control)
	}
	if scope.OptionsSource == nil || scope.OptionsSource.Params["kind"] != "namespace" {
		t.Errorf("namespace scope should source options from the namespace list, got %+v", scope.OptionsSource)
	}
	if scope.ValueField != "name" || scope.LabelField != "name" {
		t.Errorf("namespace scope should use namespace names as option values/labels, got value=%q label=%q", scope.ValueField, scope.LabelField)
	}
	if scope.AllLabel != "All namespaces" {
		t.Errorf("namespace scope should expose an all-namespaces option, got %q", scope.AllLabel)
	}
}

func TestWatchFrameLowercasesEventType(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "p1", "namespace": "ns", "uid": "u1"},
	}}
	frame := watchFrame(kind{name: "pod", namespaced: true}, watch.Event{Type: watch.Deleted, Object: obj})
	if frame == nil || frame.Type != "deleted" {
		t.Fatalf("watch frame type: want %q, got %+v", "deleted", frame)
	}
}
