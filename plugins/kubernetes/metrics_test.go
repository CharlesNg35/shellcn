package kubernetes

import (
	"context"
	"net/http"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func nodeList() obj {
	return obj{
		"apiVersion": "v1", "kind": "NodeList",
		"items": []any{obj{
			"metadata": obj{"name": "node-a"},
			"status":   obj{"allocatable": obj{"cpu": "4", "memory": "8Gi", "pods": "110"}},
		}},
	}
}

func TestClusterFrameWithMetrics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, nodeList()) })
	mux.HandleFunc("/api/v1/pods", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{"apiVersion": "v1", "kind": "PodList", "items": []any{obj{"metadata": obj{"name": "p1"}}}})
	})
	mux.HandleFunc("/apis/metrics.k8s.io/v1beta1/nodes", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "metrics.k8s.io/v1beta1", "kind": "NodeMetricsList",
			"items": []any{obj{"metadata": obj{"name": "node-a"}, "usage": obj{"cpu": "2", "memory": "4Gi"}}},
		})
	})
	sess := connectTo(t, mux).(*Session)

	frame := sess.clusterFrame(context.Background())
	if frame["metricsAvailable"] != true {
		t.Fatalf("expected metrics available: %+v", frame)
	}
	if frame["nodes"] != 1 || frame["pods"] != 1 {
		t.Fatalf("counts = %+v", frame)
	}
	if cpuPct, _ := frame["cpuPct"].(float64); cpuPct < 49 || cpuPct > 51 {
		t.Fatalf("cpuPct = %v, want ~50", frame["cpuPct"])
	}
}

func TestClusterFrameDegradesWithoutMetricsServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, nodeList()) })
	// no metrics.k8s.io endpoint, no pods endpoint
	sess := connectTo(t, mux).(*Session)

	frame := sess.clusterFrame(context.Background())
	if frame["metricsAvailable"] != false {
		t.Fatalf("metrics should be unavailable: %+v", frame)
	}
	if frame["nodes"] != 1 {
		t.Fatalf("node list should still work: %+v", frame)
	}
}

func TestNodePods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/pods", func(w http.ResponseWriter, r *http.Request) {
		if fs := r.URL.Query().Get("fieldSelector"); fs != "spec.nodeName=node-a" {
			t.Errorf("fieldSelector = %q", fs)
		}
		writeJSON(w, obj{"apiVersion": "v1", "kind": "PodList", "items": []any{
			obj{"metadata": obj{"name": "p1", "namespace": "default"}, "spec": obj{"nodeName": "node-a"}, "status": obj{"phase": "Running"}},
		}})
	})
	sess := connectTo(t, mux)

	out, err := NodePods(rc(sess, map[string]string{"name": "node-a"}))
	if err != nil {
		t.Fatalf("node pods: %v", err)
	}
	if got := len(out.(plugin.Page[Row]).Items); got != 1 {
		t.Fatalf("rows = %d", got)
	}
}
