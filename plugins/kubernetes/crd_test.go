package kubernetes

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
