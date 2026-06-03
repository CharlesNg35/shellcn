package kubernetes

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestGetYAML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/configmaps/cfg", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": obj{"name": "cfg", "namespace": "default", "managedFields": []any{obj{"manager": "x"}}},
			"data":     obj{"key": "value"},
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
	if strings.Contains(yamlStr, "managedFields") {
		t.Fatalf("managedFields should be stripped: %q", yamlStr)
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
