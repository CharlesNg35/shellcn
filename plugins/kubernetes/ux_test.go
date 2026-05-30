package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/charlesng35/shellcn/internal/plugin"
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
		add(r.ActionIDs)
		add(r.RowActionIDs)
		add(r.ListActionIDs)
		add(r.Detail.Header.ActionIDs)
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

func TestShellPodIsFixedAndReusable(t *testing.T) {
	p := shellPod()
	if p.Name != shellPodName {
		t.Errorf("shell pod needs a fixed name for reuse, got %q", p.Name)
	}
	if p.Labels[shellPodLabel] != "true" {
		t.Error("shell pod should be labelled as a managed cluster shell")
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

func hasNamespaceFilter(r plugin.ResourceType) bool {
	for _, f := range r.Filters {
		if f.Param == "namespace" {
			return true
		}
	}
	return false
}

func TestNamespaceFilterOnlyOnNamespacedKinds(t *testing.T) {
	byKind := map[string]plugin.ResourceType{}
	for _, r := range resources() {
		byKind[r.Kind] = r
	}
	if !hasNamespaceFilter(byKind["pod"]) {
		t.Error("namespaced kind (pod) should expose a namespace filter")
	}
	if hasNamespaceFilter(byKind["node"]) {
		t.Error("cluster-scoped kind (node) should not expose a namespace filter")
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
