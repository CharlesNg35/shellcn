package kubernetes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestGetYAML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/configmaps/cfg", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": obj{
				"name": "cfg", "namespace": "default",
				"resourceVersion": "100", "uid": "u-cfg", "generation": int64(3),
				"creationTimestamp": "2026-06-05T10:11:12Z",
				"managedFields":     []any{obj{"manager": "x"}},
				"annotations":       obj{"kubectl.kubernetes.io/last-applied-configuration": "{}", "keep": "me"},
			},
			"data":   obj{"key": "value"},
			"status": obj{"phase": "ok"},
		})
	})
	sess := connectTo(t, mux)

	out, err := GetYAML(rc(sess, map[string]string{"kind": "configmap", "namespace": "default", "name": "cfg"}))
	if err != nil {
		t.Fatalf("get yaml: %v", err)
	}
	yamlStr, ok := out.(string)
	if !ok {
		t.Fatalf("GetYAML returned %T, want string", out)
	}
	if !strings.Contains(yamlStr, "kind: ConfigMap") || !strings.Contains(yamlStr, "name: cfg") {
		t.Fatalf("yaml = %q", yamlStr)
	}
	// Server-managed fields must be stripped so a re-apply carries no stale
	// optimistic-concurrency precondition (the second-save bug).
	for _, banned := range []string{"resourceVersion", "managedFields", "status:", "uid:", "generation:", "creationTimestamp", "last-applied-configuration"} {
		if strings.Contains(yamlStr, banned) {
			t.Fatalf("%q should be stripped for edit: %q", banned, yamlStr)
		}
	}
	if !strings.Contains(yamlStr, "keep: me") {
		t.Fatalf("non-server annotations should be kept: %q", yamlStr)
	}
}

// TestYAMLEditRoundTrip proves the load→save→save cycle is stable: GetYAML strips
// resourceVersion, the apply body carries none (so SSA has no precondition to fail
// on a re-save), and the apply returns clean canonical content for the editor.
func TestYAMLEditRoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{"kind": "APIVersions", "versions": []any{"v1"}})
	})
	mux.HandleFunc("/apis", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{"kind": "APIGroupList", "groups": []any{}})
	})
	mux.HandleFunc("/api/v1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"kind": "APIResourceList", "groupVersion": "v1",
			"resources": []any{obj{"name": "configmaps", "namespaced": true, "kind": "ConfigMap"}},
		})
	})
	var patchBodies []map[string]any
	rv := 100
	mux.HandleFunc("/api/v1/namespaces/default/configmaps/cfg", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			body, _ := io.ReadAll(r.Body)
			var doc map[string]any
			_ = json.Unmarshal(body, &doc)
			patchBodies = append(patchBodies, doc)
			rv++
		}
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": obj{
				"name": "cfg", "namespace": "default",
				"resourceVersion": strconv.Itoa(rv),
				"managedFields":   []any{obj{"manager": "shellcn"}},
			},
			"data":   obj{"key": "value"},
			"status": obj{"observedGeneration": int64(1)},
		})
	})
	sess := connectTo(t, mux)

	loaded, err := GetYAML(rc(sess, map[string]string{"kind": "configmap", "namespace": "default", "name": "cfg"}))
	if err != nil {
		t.Fatalf("get yaml: %v", err)
	}
	content := loaded.(string)

	apply := func(c string) map[string]any {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"content": c})
		res, err := ApplyYAML(plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, body))
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		return res.(map[string]any)
	}

	// Two consecutive saves of the editor content must both succeed.
	first := apply(content)
	second := apply(first["content"].(string))

	for i, doc := range patchBodies {
		meta, _ := doc["metadata"].(map[string]any)
		if _, ok := meta["resourceVersion"]; ok {
			t.Fatalf("apply #%d body carried a resourceVersion precondition: %+v", i+1, meta)
		}
	}
	for _, res := range []map[string]any{first, second} {
		c, _ := res["content"].(string)
		if c == "" || strings.Contains(c, "resourceVersion") || strings.Contains(c, "managedFields") || strings.Contains(c, "status:") {
			t.Fatalf("apply returned unclean content: %q", c)
		}
	}
}

func TestDecodeManifestsSplitsDocuments(t *testing.T) {
	stream := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n" +
		"---\n# a comment-only document is skipped\n" +
		"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: b\n"
	docs, err := decodeManifests(stream)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("want 2 documents, got %d: %+v", len(docs), docs)
	}
	if docs[0]["kind"] != "ConfigMap" || docs[1]["kind"] != "Secret" {
		t.Errorf("documents = %+v", docs)
	}
	if empty, _ := decodeManifests("\n---\n# nothing\n"); len(empty) != 0 {
		t.Errorf("a stream of blanks should yield no documents, got %+v", empty)
	}
}

func TestApplyYAMLRejectsInvalid(t *testing.T) {
	sess := connectTo(t, http.NewServeMux())
	apply := func(content string) error {
		body, _ := json.Marshal(map[string]any{"content": content})
		rcx := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, body)
		_, err := ApplyYAML(rcx)
		return err
	}
	if err := apply(""); err == nil {
		t.Fatal("empty manifest should be rejected")
	}
	if err := apply("apiVersion: v1\n# no kind or name\n"); err == nil {
		t.Fatal("manifest without kind/name should be rejected")
	}
}
