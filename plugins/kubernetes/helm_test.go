package kubernetes

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func helmSecretPayload(t *testing.T, relJSON string) []byte {
	t.Helper()
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte(relJSON)); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	// Secret.Data["release"] holds base64(gzip(json)); the typed client has
	// already base64-decoded the Secret's outer layer.
	return []byte(base64.StdEncoding.EncodeToString(gz.Bytes()))
}

func TestDecodeHelmRelease(t *testing.T) {
	payload := helmSecretPayload(t, `{"name":"foo","namespace":"bar","version":3,"info":{"status":"deployed"},"chart":{"metadata":{"name":"foo","version":"1.2.3","appVersion":"4.5"}}}`)
	rel, err := decodeHelmRelease(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rel.Name != "foo" || rel.Version != 3 || rel.Info.Status != "deployed" || rel.Chart.Metadata.Version != "1.2.3" {
		t.Fatalf("release = %+v", rel)
	}
}

func TestHelmReleasesKeepsLatestRevision(t *testing.T) {
	secretName := func(rev int) string { return fmt.Sprintf("sh.helm.release.v1.web.v%d", rev) }
	secret := func(rev int) obj {
		rel := fmt.Sprintf(`{"name":"web","namespace":"prod","version":%d,"info":{"status":"deployed"},"chart":{"metadata":{"name":"web","version":"1.%d.0","appVersion":"4.5"}}}`, rev, rev)
		return obj{
			"metadata": obj{"name": secretName(rev), "namespace": "prod", "labels": obj{
				"owner": "helm", "name": "web", "version": strconv.Itoa(rev), "status": "deployed",
			}},
			"data": obj{"release": base64.StdEncoding.EncodeToString(helmSecretPayload(t, rel))},
		}
	}

	var fetched []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/prod/secrets", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("labelSelector"); !strings.Contains(got, "owner=helm") {
			t.Errorf("labelSelector = %q, want the helm owner selector", got)
		}
		if got := r.URL.Query().Get("limit"); got == "" {
			t.Error("release Secrets must be listed with a limit")
		}
		// Only ObjectMeta is asked for, so the gzipped payloads stay on the server.
		if got := r.Header.Get("Accept"); !strings.Contains(got, "PartialObjectMetadataList") {
			t.Errorf("Accept = %q, want PartialObjectMetadataList", got)
		}
		writeJSON(w, obj{"apiVersion": "v1", "kind": "SecretList", "items": []any{secret(1), secret(2)}})
	})
	for _, rev := range []int{1, 2} {
		mux.HandleFunc("/api/v1/namespaces/prod/secrets/"+secretName(rev), func(w http.ResponseWriter, r *http.Request) {
			fetched = append(fetched, r.URL.Path)
			writeJSON(w, secret(rev))
		})
	}
	sess := connectTo(t, mux)

	out, err := HelmReleases(rc(sess, map[string]string{"namespace": "prod"}))
	if err != nil {
		t.Fatalf("releases: %v", err)
	}
	items := out.(plugin.Page[Row]).Items
	if len(items) != 1 || items[0]["revision"] != int64(2) {
		t.Fatalf("expected one release at latest revision 2: %+v", items)
	}
	if items[0]["chart"] != "web-1.2.0" || items[0]["status"] != "deployed" || items[0]["appVersion"] != "4.5" {
		t.Fatalf("row should carry the decoded release detail: %+v", items[0])
	}
	// Superseded revisions are identified from labels and never inflated.
	if len(fetched) != 1 || !strings.HasSuffix(fetched[0], secretName(2)) {
		t.Fatalf("only the winning revision should be fetched, got %v", fetched)
	}
}

func TestHelmReleaseDetail(t *testing.T) {
	rel := `{"name":"web","namespace":"prod","version":3,"info":{"status":"deployed","notes":"hello"},"chart":{"metadata":{"name":"web","version":"1.0.0"}}}`
	body := obj{
		"metadata": obj{"name": "sh.helm.release.v1.web.v3", "namespace": "prod", "labels": obj{
			"owner": "helm", "name": "web", "version": "3", "status": "deployed",
		}},
		"data": obj{"release": base64.StdEncoding.EncodeToString(helmSecretPayload(t, rel))},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/prod/secrets", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("labelSelector"); !strings.Contains(got, "name=web") {
			t.Errorf("detail should narrow to one release, labelSelector = %q", got)
		}
		writeJSON(w, obj{"apiVersion": "v1", "kind": "SecretList", "items": []any{body}})
	})
	mux.HandleFunc("/api/v1/namespaces/prod/secrets/sh.helm.release.v1.web.v3", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, body)
	})
	sess := connectTo(t, mux)

	out, err := HelmRelease(rc(sess, map[string]string{"namespace": "prod", "name": "web"}))
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	got := out.(map[string]any)
	if got["revision"] != 3 || got["notes"] != "hello" {
		t.Fatalf("release detail = %+v", got)
	}
}
