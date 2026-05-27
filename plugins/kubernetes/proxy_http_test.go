package kubernetes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestAPIProxyPath(t *testing.T) {
	cases := map[string]string{
		"/services/default/web/80/":         "/api/v1/namespaces/default/services/web:80/proxy/",
		"/services/mon/graf/3000/d/abc?x=1": "/api/v1/namespaces/mon/services/graf:3000/proxy/d/abc?x=1",
		"/pods/default/api/8080/healthz":    "/api/v1/namespaces/default/pods/api:8080/proxy/healthz",
	}
	for in, want := range cases {
		got, ok := apiProxyPath(in)
		if !ok || got != want {
			t.Errorf("apiProxyPath(%q) = %q,%v; want %q", in, got, ok, want)
		}
	}
	if _, ok := apiProxyPath("/configmaps/default/cm"); ok {
		t.Error("non-proxyable kind should be rejected")
	}
}

func TestServiceProxyURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "web", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"port": int64(8080)}}},
		})
	})
	sess := connectTo(t, mux)

	out, err := ServiceProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "web"}))
	if err != nil {
		t.Fatalf("proxy url: %v", err)
	}
	url, _ := out.(map[string]any)["url"].(string)
	if url != "/api/connections/c1/proxy/services/default/web/8080/" {
		t.Fatalf("url = %q", url)
	}
}

func TestServeHTTPProxyForwardsToServiceProxy(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web:80/proxy/hello", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hi from the service")
	})
	sess := connectTo(t, mux).(*Session)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/services/default/web/80/hello", nil)
	sess.ServeHTTPProxy(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hi from the service") {
		t.Fatalf("proxy response = %d %q", rec.Code, rec.Body.String())
	}
}

func TestServiceOpenActionIsURLTarget(t *testing.T) {
	for _, a := range New().Manifest().Actions {
		if a.ID == "kubernetes.service.open" {
			if a.Open != plugin.OpenURL {
				t.Fatalf("service open action should use OpenURL, got %q", a.Open)
			}
			return
		}
	}
	t.Fatal("kubernetes.service.open action not declared")
}
