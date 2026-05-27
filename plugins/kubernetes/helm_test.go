package kubernetes

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
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
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/prod/secrets", func(w http.ResponseWriter, _ *http.Request) {
		mk := func(rev int) obj {
			rel := `{"name":"web","namespace":"prod","version":` + string(rune('0'+rev)) + `,"info":{"status":"deployed"},"chart":{"metadata":{"name":"web","version":"1.0.0"}}}`
			return obj{"metadata": obj{"name": "sh.helm.release.v1.web.v" + string(rune('0'+rev))}, "data": obj{"release": base64.StdEncoding.EncodeToString(helmSecretPayload(t, rel))}}
		}
		writeJSON(w, obj{"apiVersion": "v1", "kind": "SecretList", "items": []any{mk(1), mk(2)}})
	})
	sess := connectTo(t, mux)

	out, err := HelmReleases(rc(sess, map[string]string{"namespace": "prod"}))
	if err != nil {
		t.Fatalf("releases: %v", err)
	}
	items := out.(plugin.Page[Row]).Items
	if len(items) != 1 || items[0]["revision"] != int64(2) {
		t.Fatalf("expected one release at latest revision 2: %+v", items)
	}
}

func TestProxyExecuteRejectsNonAPIPath(t *testing.T) {
	sess := connectTo(t, http.NewServeMux())
	rcx := plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, sess, nil, nil, []byte(`{"method":"GET","url":"http://evil.example/"}`))
	if _, err := ProxyExecute(rcx); err == nil {
		t.Fatal("proxy must reject non-API-server paths")
	}
}
