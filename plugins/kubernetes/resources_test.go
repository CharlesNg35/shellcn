package kubernetes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
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
	return plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, params, url.Values{}, nil).
		WithProxyPrefix("/api/connections/c1/proxy")
}

func TestListResourceNamespacedPods(t *testing.T) {
	const created = "2026-06-05T10:11:12Z"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "PodList",
			"items": []any{
				obj{
					"metadata": obj{"name": "web-1", "namespace": "default", "uid": "u1", "creationTimestamp": created},
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
	if r["age"] != created || r["createdAt"] != created {
		t.Fatalf("pod age fields = %+v", r)
	}
	// Every list row must carry a ref so the grid can open detail + row actions.
	ref, ok := r["ref"].(plugin.ResourceIdentity)
	if !ok || ref.Kind != "pod" || ref.Name != "web-1" || ref.Namespace != "default" {
		t.Fatalf("pod row ref = %+v (ok=%v)", r["ref"], ok)
	}
}

func TestResourceOverviewPodIsStructured(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods/web-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": obj{
				"name":              "web-1",
				"namespace":         "default",
				"uid":               "u1",
				"creationTimestamp": "2026-06-05T10:11:12Z",
				"labels":            obj{"app": "web"},
			},
			"spec": obj{
				"nodeName":           "node-a",
				"serviceAccountName": "web-sa",
				"containers": []any{obj{
					"name": "app",
					"resources": obj{
						"requests": obj{"cpu": "250m", "memory": "128Mi"},
						"limits":   obj{"cpu": "500m", "memory": "256Mi"},
					},
				}},
				"volumes": []any{obj{"name": "config"}},
			},
			"status": obj{
				"phase":    "Running",
				"podIP":    "10.1.2.3",
				"hostIP":   "192.168.1.10",
				"qosClass": "Burstable",
				"containerStatuses": []any{
					obj{"name": "app", "ready": true, "restartCount": int64(2)},
				},
			},
		})
	})
	sess := connectTo(t, mux)

	out, err := ResourceOverview(rc(sess, map[string]string{"kind": "pod", "namespace": "default", "name": "web-1"}))
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	row := out.(Row)
	if row["status"] != "Running" || row["ready"] != "1/1" || row["node"] != "node-a" || row["serviceAccount"] != "web-sa" {
		t.Fatalf("pod overview summary = %+v", row)
	}
	if row["cpuRequest"] != 0.25 || row["cpuLimit"] != 0.5 {
		t.Fatalf("pod overview cpu = %+v", row)
	}
	if row["memRequest"] != int64(134217728) || row["memLimit"] != int64(268435456) {
		t.Fatalf("pod overview memory = %+v", row)
	}
	if row["volumes"] != int64(1) || row["restarts"] != int64(2) {
		t.Fatalf("pod overview counters = %+v", row)
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

func TestBuiltInAgeColumnsUseRelativeTime(t *testing.T) {
	for _, k := range kinds {
		for _, c := range k.columns {
			if c.Key == "age" && c.Type != plugin.ColumnRelativeTime {
				t.Fatalf("%s age column type = %q, want %q", k.name, c.Type, plugin.ColumnRelativeTime)
			}
		}
	}
}

func TestBuiltInResourceImportantColumns(t *testing.T) {
	cases := map[string][]string{
		"storageclass":          {"default", "provisioner", "reclaim", "bindingMode", "allowExpand"},
		"daemonset":             {"desired", "current", "ready", "upToDate", "available"},
		"service":               {"type", "clusterIP", "externalIP", "ports"},
		"endpoints":             {"endpoints"},
		"ingress":               {"class", "hosts", "address", "ports"},
		"networkpolicy":         {"podSelector", "policyTypes"},
		"persistentvolumeclaim": {"status", "volume", "capacity", "accessModes", "storageClass"},
		"persistentvolume":      {"capacity", "accessModes", "status", "claim", "storageClass", "reclaim", "reason"},
		"poddisruptionbudget":   {"minAvailable", "maxUnavailable", "allowedDisruptions"},
		"job":                   {"completions", "duration", "active"},
		"cronjob":               {"schedule", "timezone", "suspend", "active", "lastSchedule"},
		"ingressclass":          {"controller", "parameters"},
	}

	for kindName, keys := range cases {
		k, ok := kindByName(kindName)
		if !ok {
			t.Fatalf("missing kind %q", kindName)
		}
		have := map[string]bool{}
		for _, c := range k.columns {
			have[c.Key] = true
		}
		for _, key := range keys {
			if !have[key] {
				t.Fatalf("%s missing column %q in %+v", kindName, key, k.columns)
			}
		}
	}
}

func TestStorageClassDefaultRow(t *testing.T) {
	row := storageClassRow(obj{
		"metadata": obj{"annotations": obj{"storageclass.kubernetes.io/is-default-class": "true"}},
	})
	if row["default"] != true {
		t.Fatalf("default storage class row = %+v", row)
	}

	row = storageClassRow(obj{
		"metadata": obj{"annotations": obj{"storageclass.beta.kubernetes.io/is-default-class": "true"}},
	})
	if row["default"] != true {
		t.Fatalf("beta default storage class row = %+v", row)
	}
}

func TestOperationalRowDetails(t *testing.T) {
	service := serviceRow(obj{
		"spec": obj{
			"type":        "LoadBalancer",
			"clusterIP":   "10.0.0.1",
			"externalIPs": []any{"203.0.113.10"},
			"ports":       []any{obj{"port": int64(443), "protocol": "TCP"}},
		},
		"status": obj{"loadBalancer": obj{"ingress": []any{obj{"hostname": "lb.example.test"}}}},
	})
	if service["externalIP"] != "203.0.113.10, lb.example.test" || service["ports"] != "443/TCP" {
		t.Fatalf("service row = %+v", service)
	}

	ingress := ingressRow(obj{
		"spec": obj{
			"ingressClassName": "nginx",
			"tls":              []any{obj{}},
			"rules":            []any{obj{"host": "app.example.test"}},
		},
		"status": obj{"loadBalancer": obj{"ingress": []any{obj{"ip": "198.51.100.20"}}}},
	})
	if ingress["address"] != "198.51.100.20" || ingress["ports"] != "80, 443" {
		t.Fatalf("ingress row = %+v", ingress)
	}

	networkPolicy := networkPolicyRow(obj{
		"spec": obj{
			"podSelector": obj{"matchLabels": obj{"app": "api"}},
			"policyTypes": []any{"Ingress", "Egress"},
		},
	})
	if networkPolicy["podSelector"] != "app=api" || networkPolicy["policyTypes"] != "Ingress, Egress" {
		t.Fatalf("network policy row = %+v", networkPolicy)
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

	// Rows are keyed by those column names + carry a customresource identity.
	listOut, err := ListResource(rc(sess, map[string]string{"kind": crd}))
	if err != nil {
		t.Fatalf("list crd: %v", err)
	}
	row := listOut.(plugin.Page[Row]).Items[0]
	if row["Name"] != "w1" || row["Phase"] != "Ready" {
		t.Fatalf("crd row cells = %+v", row)
	}
	ref, ok := row["ref"].(plugin.ResourceIdentity)
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
