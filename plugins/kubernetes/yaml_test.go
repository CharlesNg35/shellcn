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

// serviceEditMux is a fake apiserver hosting one Service with two named ports. It
// records every write (method, query, parsed body) so a test can assert apply uses
// replace (PUT) semantics rather than server-side-apply merging.
func serviceEditMux(t *testing.T, writes *[]editWrite, rv *int) *http.ServeMux {
	t.Helper()
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
			"resources": []any{obj{"name": "services", "namespaced": true, "kind": "Service"}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/services/shellcn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut || r.Method == http.MethodPatch {
			raw, _ := io.ReadAll(r.Body)
			var doc map[string]any
			_ = json.Unmarshal(raw, &doc)
			*writes = append(*writes, editWrite{method: r.Method, query: r.URL.RawQuery, body: doc})
			if !strings.Contains(r.URL.RawQuery, "dryRun") {
				*rv++
			}
		}
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service",
			"metadata": obj{
				"name": "shellcn", "namespace": "default",
				"resourceVersion": strconv.Itoa(*rv),
				"managedFields":   []any{obj{"manager": "shellcn"}},
			},
			"spec": obj{"ports": []any{
				obj{"port": int64(80), "name": "web"},
				obj{"port": int64(443), "name": "https"},
			}},
			"status": obj{"loadBalancer": obj{}},
		})
	})
	return mux
}

type editWrite struct {
	method string
	query  string
	body   map[string]any
}

// TestYAMLEditRoundTrip proves the load→save→save cycle is stable and uses replace
// (PUT) semantics: GetYAML strips server fields; apply re-reads the live
// resourceVersion (so a re-save never fails an optimistic-concurrency precondition);
// and apply returns clean canonical content.
func TestYAMLEditRoundTrip(t *testing.T) {
	var writes []editWrite
	rv := 100
	sess := connectTo(t, serviceEditMux(t, &writes, &rv))

	loaded, err := GetYAML(rc(sess, map[string]string{"kind": "service", "namespace": "default", "name": "shellcn"}))
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

	first := apply(content)
	second := apply(first["content"].(string))

	if len(writes) != 2 {
		t.Fatalf("want 2 writes, got %d", len(writes))
	}
	for i, wr := range writes {
		if wr.method != http.MethodPut {
			t.Fatalf("write #%d used %s, want PUT (replace) semantics", i+1, wr.method)
		}
		meta, _ := wr.body["metadata"].(map[string]any)
		if meta["resourceVersion"] != strconv.Itoa(100+i) {
			t.Fatalf("write #%d must carry the freshly-read resourceVersion, got %v", i+1, meta["resourceVersion"])
		}
	}
	for _, res := range []map[string]any{first, second} {
		c, _ := res["content"].(string)
		if c == "" || strings.Contains(c, "managedFields") || strings.Contains(c, "status:") {
			t.Fatalf("apply returned unclean content: %q", c)
		}
	}
}

// TestYAMLApplyReplacesPorts is the regression for the duplicate-port bug: renaming a
// Service port and applying must replace the whole ports list (no server-side-apply
// associative merge that would collide two entries on the same name).
func TestYAMLApplyReplacesPorts(t *testing.T) {
	var writes []editWrite
	rv := 100
	sess := connectTo(t, serviceEditMux(t, &writes, &rv))

	loaded, err := GetYAML(rc(sess, map[string]string{"kind": "service", "namespace": "default", "name": "shellcn"}))
	if err != nil {
		t.Fatalf("get yaml: %v", err)
	}
	edited := strings.Replace(loaded.(string), "name: web", "name: http", 1)

	body, _ := json.Marshal(map[string]any{"content": edited})
	if _, err := ApplyYAML(plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, body)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(writes) != 1 || writes[0].method != http.MethodPut {
		t.Fatalf("want one PUT, got %+v", writes)
	}
	spec, _ := writes[0].body["spec"].(map[string]any)
	ports, _ := spec["ports"].([]any)
	names := make([]string, 0, len(ports))
	for _, p := range ports {
		pm, _ := p.(map[string]any)
		names = append(names, pm["name"].(string))
	}
	if strings.Join(names, ",") != "http,https" {
		t.Fatalf("replaced ports = %v, want [http https] with no duplicate", names)
	}
}

// TestYAMLDryRunThreadsFlag proves Preview reaches the apiserver as dryRun=All and
// returns content the editor can diff.
func TestYAMLDryRunThreadsFlag(t *testing.T) {
	var writes []editWrite
	rv := 100
	sess := connectTo(t, serviceEditMux(t, &writes, &rv))

	loaded, _ := GetYAML(rc(sess, map[string]string{"kind": "service", "namespace": "default", "name": "shellcn"}))
	body, _ := json.Marshal(map[string]any{"content": loaded.(string), "dryRun": true})
	res, err := ApplyYAML(plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, body))
	if err != nil {
		t.Fatalf("dry-run apply: %v", err)
	}
	if len(writes) != 1 || !strings.Contains(writes[0].query, "dryRun=All") {
		t.Fatalf("dry-run not threaded as dryRun=All: %+v", writes)
	}
	if c, _ := res.(map[string]any)["content"].(string); c == "" {
		t.Fatal("dry-run must return content for the preview diff")
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
