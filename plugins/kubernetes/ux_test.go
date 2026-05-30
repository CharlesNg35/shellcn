package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestNoActionsOpenDock(t *testing.T) {
	for _, a := range actions() {
		if a.Open == plugin.OpenDock {
			t.Errorf("action %q opens a dock; it should be a detail tab or a dialog", a.ID)
		}
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
