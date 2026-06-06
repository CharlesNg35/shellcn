package kubernetes

import (
	"context"
	"net/http"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
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

func TestPodFrameIncludesRequestsLimitsAndUsage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   obj{"name": "web", "namespace": "default"},
			"spec": obj{"containers": []any{
				obj{"name": "app", "resources": obj{
					"requests": obj{"cpu": "250m", "memory": "128Mi"},
					"limits":   obj{"cpu": "500m", "memory": "256Mi"},
				}},
				obj{"name": "sidecar", "resources": obj{
					"requests": obj{"cpu": "100m", "memory": "64Mi"},
					"limits":   obj{"cpu": "200m", "memory": "128Mi"},
				}},
			}},
		})
	})
	mux.HandleFunc("/apis/metrics.k8s.io/v1beta1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "metrics.k8s.io/v1beta1",
			"kind":       "PodMetrics",
			"metadata":   obj{"name": "web", "namespace": "default"},
			"containers": []any{
				obj{"name": "app", "usage": obj{"cpu": "200m", "memory": "100Mi"}},
				obj{"name": "sidecar", "usage": obj{"cpu": "50m", "memory": "28Mi"}},
			},
		})
	})
	sess := connectTo(t, mux).(*Session)

	frame := sess.podFrame(context.Background(), "default", "web")
	if frame["metricsAvailable"] != true {
		t.Fatalf("metrics should be available: %+v", frame)
	}
	if frame["cpuRequest"] != 0.35 || frame["cpuLimit"] != 0.7 {
		t.Fatalf("cpu request/limit = %+v", frame)
	}
	if frame["memRequest"] != int64(201326592) || frame["memLimit"] != int64(402653184) {
		t.Fatalf("memory request/limit = %+v", frame)
	}
	if frame["mem"] != int64(134217728) {
		t.Fatalf("memory usage = %+v", frame)
	}
	if pct, _ := frame["memLimitPct"].(float64); pct < 33 || pct > 34 {
		t.Fatalf("memLimitPct = %+v", frame)
	}
}

func TestPodFrameDegradesWithoutMetrics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   obj{"name": "web", "namespace": "default"},
			"spec": obj{"containers": []any{
				obj{"name": "app", "resources": obj{"requests": obj{"memory": "64Mi"}}},
			}},
		})
	})
	sess := connectTo(t, mux).(*Session)

	frame := sess.podFrame(context.Background(), "default", "web")
	if frame["metricsAvailable"] != false {
		t.Fatalf("metrics should be unavailable: %+v", frame)
	}
	if frame["memRequest"] != int64(67108864) {
		t.Fatalf("request should still be shown: %+v", frame)
	}
}
