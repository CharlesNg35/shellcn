package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

func connectTo(t *testing.T, mux *http.ServeMux) plugin.Session {
	t.Helper()
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"major": "1", "minor": "31"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect,
		Config: map[string]any{"kubeconfig": kubeconfigFor(srv.URL)}, Net: fakeNet{},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func rc(sess plugin.Session, params map[string]string) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sess, params, url.Values{}, nil)
}

func TestListResourceNamespacedPods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "PodList",
			"items": []any{
				obj{
					"metadata": obj{"name": "web-1", "namespace": "default", "uid": "u1"},
					"spec":     obj{"nodeName": "node-a"},
					"status": obj{
						"phase":             "Running",
						"podIP":             "10.1.2.3",
						"containerStatuses": []any{obj{"ready": true, "restartCount": int64(2)}},
					},
				},
			},
		})
	})
	sess := connectTo(t, mux)

	out, err := ListResource(rc(sess, map[string]string{"kind": "pod", "namespace": "default"}))
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	page := out.(plugin.Page[Row])
	if len(page.Items) != 1 {
		t.Fatalf("rows = %d", len(page.Items))
	}
	r := page.Items[0]
	if r["name"] != "web-1" || r["status"] != "Running" || r["ready"] != "1/1" || r["node"] != "node-a" {
		t.Fatalf("pod row = %+v", r)
	}
}

func TestDeleteResource(t *testing.T) {
	deleted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/configmaps/cfg", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = true
		}
		writeJSON(w, obj{"apiVersion": "v1", "kind": "Status", "status": "Success"})
	})
	sess := connectTo(t, mux)

	if _, err := DeleteResource(rc(sess, map[string]string{"kind": "configmap", "namespace": "default", "name": "cfg"})); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Fatal("expected a DELETE to the configmap")
	}
}

func TestTreeCategoryListsKinds(t *testing.T) {
	out, err := TreeCategory(rc(nil, map[string]string{"category": "workloads"}))
	if err != nil {
		t.Fatalf("tree category: %v", err)
	}
	page := out.(plugin.Page[plugin.TreeNode])
	var foundPod bool
	for _, n := range page.Items {
		if n.ResourceKind == "" || !n.Leaf {
			t.Fatalf("category node should open a kind list: %+v", n)
		}
		if n.ResourceKind == "pod" {
			foundPod = true
		}
	}
	if !foundPod {
		t.Fatal("workloads category should include Pods")
	}
}

func TestResolveKindCRD(t *testing.T) {
	mux := http.NewServeMux()
	// Discovery for the CRD's group/version (used to learn scope).
	mux.HandleFunc("/apis/example.com/v1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"kind": "APIResourceList", "groupVersion": "example.com/v1",
			"resources": []any{obj{"name": "widgets", "namespaced": true, "kind": "Widget"}},
		})
	})
	sess := connectTo(t, mux).(*Session)

	k, err := resolveKind(sess, "crd:example.com/v1/widgets")
	if err != nil {
		t.Fatalf("resolve crd: %v", err)
	}
	if k.gvr.Group != "example.com" || k.gvr.Resource != "widgets" || !k.namespaced {
		t.Fatalf("crd kind = %+v", k)
	}
}
