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
	// Every list row must carry a ref so the grid can open detail + row actions.
	ref, ok := r["ref"].(plugin.ResourceRef)
	if !ok || ref.Kind != "pod" || ref.Name != "web-1" || ref.Namespace != "default" {
		t.Fatalf("pod row ref = %+v (ok=%v)", r["ref"], ok)
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

func TestCRDDynamicColumns(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/example.com/v1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"kind": "APIResourceList", "groupVersion": "example.com/v1",
			"resources": []any{obj{"name": "widgets", "namespaced": true, "kind": "Widget"}},
		})
	})
	mux.HandleFunc("/apis/example.com/v1/widgets", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"kind": "Table", "apiVersion": "meta.k8s.io/v1",
			"columnDefinitions": []any{obj{"name": "Name", "type": "string"}, obj{"name": "Phase", "type": "string"}},
			"rows": []any{obj{
				"cells":  []any{"w1", "Ready"},
				"object": obj{"kind": "PartialObjectMetadata", "metadata": obj{"name": "w1", "namespace": "default", "uid": "u-w1"}},
			}},
		})
	})
	sess := connectTo(t, mux)
	crd := "crd:example.com/v1/widgets"

	// Dynamic columns come from the server's Table definitions.
	colsOut, err := ColumnsForKind(rc(sess, map[string]string{"kind": crd}))
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	cols := colsOut.(plugin.Page[Row]).Items
	if len(cols) != 2 || cols[0]["name"] != "Name" || cols[1]["name"] != "Phase" {
		t.Fatalf("crd columns = %+v", cols)
	}

	// Rows are keyed by those column names + carry a customresource ref.
	listOut, err := ListResource(rc(sess, map[string]string{"kind": crd}))
	if err != nil {
		t.Fatalf("list crd: %v", err)
	}
	row := listOut.(plugin.Page[Row]).Items[0]
	if row["Name"] != "w1" || row["Phase"] != "Ready" {
		t.Fatalf("crd row cells = %+v", row)
	}
	ref, ok := row["ref"].(plugin.ResourceRef)
	if !ok || ref.Kind != customResourceKind || ref.Scope != crd || ref.Name != "w1" {
		t.Fatalf("crd row ref = %+v", row["ref"])
	}
}

func TestTreeCategoryNestsSubgroups(t *testing.T) {
	out, err := TreeCategory(rc(nil, map[string]string{"category": "config"}))
	if err != nil {
		t.Fatalf("tree config: %v", err)
	}
	var sub *plugin.TreeNode
	for i, n := range out.(plugin.Page[plugin.TreeNode]).Items {
		if n.Label == "Admission Policies" {
			sub = &out.(plugin.Page[plugin.TreeNode]).Items[i]
		}
		if n.ResourceKind == "validatingadmissionpolicy" {
			t.Fatal("admission policy kinds should be nested, not flat under Config")
		}
	}
	if sub == nil || sub.ChildrenSource == nil || sub.ResourceKind != "" {
		t.Fatalf("expected an expandable Admission Policies sub-group: %+v", sub)
	}

	// The sub-group expands to its kinds.
	subOut, err := TreeSubgroup(rc(nil, map[string]string{"subgroup": "admissionpolicies"}))
	if err != nil {
		t.Fatalf("subgroup: %v", err)
	}
	kinds := subOut.(plugin.Page[plugin.TreeNode]).Items
	if len(kinds) != 2 || kinds[0].ResourceKind == "" {
		t.Fatalf("admission policies subgroup = %+v", kinds)
	}

	// Network exposes a Gateway API branch.
	net, _ := TreeCategory(rc(nil, map[string]string{"category": "network"}))
	hasGW := false
	for _, n := range net.(plugin.Page[plugin.TreeNode]).Items {
		if n.Label == "Gateway API" && n.ChildrenSource != nil {
			hasGW = true
		}
	}
	if !hasGW {
		t.Fatal("network category should expose a Gateway API sub-group")
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
