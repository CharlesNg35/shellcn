package kubernetes

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func crdItem(name, group, plural, kind string) obj {
	return obj{
		"metadata": obj{"name": name},
		"spec": obj{
			"group":    group,
			"names":    obj{"plural": plural, "kind": kind},
			"versions": []any{obj{"name": "v1", "served": true, "storage": true}},
		},
	}
}

func TestTreeCRDsGroupByApiGroup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/apiextensions.k8s.io/v1/customresourcedefinitions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinitionList",
			"items": []any{
				crdItem("widgets.example.com", "example.com", "widgets", "Widget"),
				crdItem("ciliumnetworkpolicies.cilium.io", "cilium.io", "ciliumnetworkpolicies", "CiliumNetworkPolicy"),
			},
		})
	})
	sess := connectTo(t, mux).(*Session)

	out, err := TreeCRDs(rc(sess, nil))
	page, ok := mustTree(t, out, err)
	if !ok {
		return
	}
	// Definitions leaf, then one expandable folder per group (sorted).
	if len(page) != 3 {
		t.Fatalf("want Definitions + 2 group folders, got %d: %+v", len(page), page)
	}
	if page[0].Label != "Definitions" || !page[0].Leaf {
		t.Errorf("first node should be the Definitions leaf: %+v", page[0])
	}
	if page[1].Label != "cilium.io" || page[1].ChildrenSource == nil {
		t.Errorf("expected an expandable cilium.io folder, got %+v", page[1])
	}
	if page[2].Label != "example.com" {
		t.Errorf("groups should be sorted alphabetically: %+v", page)
	}

	groupOut, err := TreeCRDGroup(rc(sess, map[string]string{"group": "cilium.io"}))
	kinds, ok := mustTree(t, groupOut, err)
	if !ok {
		return
	}
	if len(kinds) != 1 || kinds[0].Label != "CiliumNetworkPolicy" || !kinds[0].Leaf {
		t.Fatalf("a group should list only its own kinds, got %+v", kinds)
	}
}

func mustTree(t *testing.T, out any, err error) ([]plugin.TreeNode, bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("tree handler: %v", err)
		return nil, false
	}
	page, ok := out.(plugin.Page[plugin.TreeNode])
	if !ok {
		t.Fatalf("handler returned %T, want plugin.Page[plugin.TreeNode]", out)
		return nil, false
	}
	return page.Items, true
}

func TestSampleObjectHonorsRequiredAndDefaults(t *testing.T) {
	spec := obj{
		"type":     "object",
		"required": []any{"size"},
		"properties": obj{
			"size":     obj{"type": "integer"},
			"mode":     obj{"type": "string", "default": "fast"},
			"replicas": obj{"type": "integer", "default": int64(3)},
			"label":    obj{"type": "string"}, // optional, no default → omitted
		},
	}
	root := obj{"type": "object", "required": []any{"spec"}, "properties": obj{"spec": spec}}

	out := sampleObject(root)
	got, ok := out["spec"].(obj)
	if !ok {
		t.Fatalf("expected a spec object, got %T (%+v)", out["spec"], out)
	}
	if got["size"] != 0 {
		t.Fatalf("required integer should seed 0, got %v", got["size"])
	}
	if got["mode"] != "fast" {
		t.Fatalf("declared default should win, got %v", got["mode"])
	}
	if got["replicas"] != int64(3) {
		t.Fatalf("declared default should win, got %v", got["replicas"])
	}
	if _, present := got["label"]; present {
		t.Fatalf("optional field without a default must be omitted: %+v", got)
	}
}

func TestCRDSkeletonDerivesFromDefinition(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/apis/apiextensions.k8s.io/v1/customresourcedefinitions/widgets.example.com", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
			"metadata": obj{"name": "widgets.example.com"},
			"spec": obj{
				"group": "example.com",
				"names": obj{"plural": "widgets", "kind": "Widget"},
				"versions": []any{obj{
					"name": "v1", "served": true, "storage": true,
					"schema": obj{"openAPIV3Schema": obj{
						"type": "object", "required": []any{"spec"},
						"properties": obj{"spec": obj{
							"type": "object", "required": []any{"size"},
							"properties": obj{"size": obj{"type": "integer"}},
						}},
					}},
				}},
			},
		})
	})
	sess := connectTo(t, mux).(*Session)

	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	body, ok := crdSkeleton(rc(sess, nil), sess, gvr)
	if !ok {
		t.Fatal("expected a schema-derived skeleton from the CRD definition")
	}
	spec, ok := body["spec"].(obj)
	if !ok || spec["size"] != 0 {
		t.Fatalf("skeleton should carry the required spec.size field: %+v", body)
	}
}
