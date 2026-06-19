package kubernetes

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestResourceWatchStreamsAreResourceKind(t *testing.T) {
	want := map[string]bool{
		"kubernetes.resource.watch":        true,
		"kubernetes.resource.object.watch": true,
		"kubernetes.resource.yaml.watch":   true,
		"kubernetes.resource.events.watch": true,
	}
	for _, s := range streams() {
		if !want[s.ID] {
			continue
		}
		if s.Kind != plugin.StreamResource {
			t.Errorf("stream %s kind = %s, want resource", s.ID, s.Kind)
		}
		delete(want, s.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing resource-watch streams: %v", want)
	}
}

func TestYAMLEditorWatchesAndRefreshes(t *testing.T) {
	ec := yamlEditorConfig(yamlWatchSource(map[string]string{"kind": "pod", "name": "x"}))
	if ec.Watch == nil || ec.Watch.RouteID != "kubernetes.resource.yaml.watch" {
		t.Fatalf("yaml editor must watch yaml.watch, got %+v", ec.Watch)
	}
	if ec.RefreshField != "content" {
		t.Fatalf("yaml editor refreshField = %q, want content", ec.RefreshField)
	}
	if ec.DryRunKey != "dryRun" {
		t.Fatalf("yaml editor dryRunKey = %q, want dryRun", ec.DryRunKey)
	}
	// The Create dialog has no live object to watch.
	if create := yamlEditorConfig(nil); create.Watch != nil {
		t.Fatal("create editor must not declare a watch")
	}
}

func TestOverviewTabWatchesObject(t *testing.T) {
	k, ok := kindByName("pod")
	if !ok {
		t.Fatal("pod kind missing")
	}
	tab := overviewTab(k, map[string]string{"kind": "pod", "name": "x"})
	cfg, ok := tab.Config.(plugin.ObjectDetailConfig)
	if !ok || cfg.Watch == nil || cfg.Watch.RouteID != "kubernetes.resource.object.watch" {
		t.Fatalf("overview tab must watch object.watch, got %+v", tab.Config)
	}
}

func TestEventsTimelineWatchesOrPolls(t *testing.T) {
	scoped := eventTimelineConfig(eventsWatchSource(map[string]string{"kind": "pod", "name": "x"}))
	if scoped.Watch == nil || scoped.Watch.RouteID != "kubernetes.resource.events.watch" {
		t.Fatalf("object events must watch events.watch, got %+v", scoped.Watch)
	}
	// Cluster-wide events have no single object, so they fall back to polling.
	cluster := eventTimelineConfig(nil)
	if cluster.Watch != nil || cluster.RefreshIntervalMs == 0 {
		t.Fatalf("cluster events should poll, got %+v", cluster)
	}
}
