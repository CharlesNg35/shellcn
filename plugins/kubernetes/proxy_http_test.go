package kubernetes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestProxyPaths(t *testing.T) {
	s := connectTo(t, http.NewServeMux()).(*Session)
	cases := map[string][2]string{
		"/services/default/web/80/":      {"/api/v1/namespaces/default/services/web:80/proxy/", "/api/connections/c1/proxy/services/default/web/80"},
		"/pods/default/api/8080/healthz": {"/api/v1/namespaces/default/pods/api:8080/proxy/healthz", "/api/connections/c1/proxy/pods/default/api/8080"},
		"/services/mon/graf/https:8443/": {"/api/v1/namespaces/mon/services/https:graf:8443/proxy/", "/api/connections/c1/proxy/services/mon/graf/https:8443"},
	}
	for in, want := range cases {
		apiPath, prefix, ok := s.proxyPaths(in)
		if !ok || apiPath != want[0] || prefix != want[1] {
			t.Errorf("proxyPaths(%q) = %q,%q,%v; want %q,%q", in, apiPath, prefix, ok, want[0], want[1])
		}
	}
	if _, _, ok := s.proxyPaths("/configmaps/default/cm"); ok {
		t.Error("non-proxyable kind should be rejected")
	}
}

func TestServeHTTPProxyRewritesHTML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web:80/proxy/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		_, _ = io.WriteString(w, `<html><head><title>x</title></head><body><a href="/dashboard">d</a></body></html>`)
	})
	sess := connectTo(t, mux).(*Session)
	rec := httptest.NewRecorder()
	sess.ServeHTTPProxy(rec, httptest.NewRequest(http.MethodGet, "/services/default/web/80/", nil))

	body := rec.Body.String()
	prefix := "/api/connections/c1/proxy/services/default/web/80"
	if !strings.Contains(body, `<base href="`+prefix+`/">`) {
		t.Fatalf("missing rewritten <base>: %s", body)
	}
	if !strings.Contains(body, `href="`+prefix+`/dashboard"`) {
		t.Fatalf("root-relative link not rewritten: %s", body)
	}
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP should be dropped so the shim/assets load")
	}
}

func TestServiceProxyURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/services/web", func(w http.ResponseWriter, _ *http.Request) {
		// A non-web port first, then http — selection must prefer the http port.
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "web", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"name": "grpc", "port": int64(9000)}, obj{"name": "http", "port": int64(8080)}}},
		})
	})
	mux.HandleFunc("/api/v1/namespaces/default/services/secure", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, obj{
			"apiVersion": "v1", "kind": "Service", "metadata": obj{"name": "secure", "namespace": "default"},
			"spec": obj{"ports": []any{obj{"name": "https", "port": int64(8443)}}},
		})
	})
	sess := connectTo(t, mux)

	out, err := ServiceProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "web"}))
	if err != nil {
		t.Fatalf("proxy url: %v", err)
	}
	if url, _ := out.(map[string]any)["url"].(string); url != "/api/connections/c1/proxy/services/default/web/8080/" {
		t.Fatalf("http port should win: %q", url)
	}

	out, err = ServiceProxyURL(rc(sess, map[string]string{"namespace": "default", "name": "secure"}))
	if err != nil {
		t.Fatalf("proxy url (https): %v", err)
	}
	if url, _ := out.(map[string]any)["url"].(string); url != "/api/connections/c1/proxy/services/default/secure/https:8443/" {
		t.Fatalf("https port segment wrong: %q", url)
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
