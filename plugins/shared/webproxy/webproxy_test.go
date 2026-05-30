package webproxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/webproxy"
)

func TestIsTLSPort(t *testing.T) {
	tls := []int{443, 8443, 9443, 10443, 4443}
	for _, p := range tls {
		if !webproxy.IsTLSPort(p) {
			t.Errorf("IsTLSPort(%d) = false, want true", p)
		}
	}
	plain := []int{80, 8080, 3000, 8000, 5000, 22}
	for _, p := range plain {
		if webproxy.IsTLSPort(p) {
			t.Errorf("IsTLSPort(%d) = true, want false", p)
		}
	}
}

// Serve proxying straight to the app (no SourcePrefix) must prefix bare
// root-relative URLs, inject the base + runtime-asset shim, and drop framing/CSP.
func TestServeRewritesHTMLUnderPrefix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		_, _ = io.WriteString(w, `<html><head><title>x</title></head><body><a href="/dashboard">d</a></body></html>`)
	}))
	defer upstream.Close()
	base, _ := url.Parse(upstream.URL)

	rec := httptest.NewRecorder()
	webproxy.Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x",
	})

	body := rec.Body.String()
	if !strings.Contains(body, `<base href="/proxy/x/">`) {
		t.Fatalf("missing rewritten <base>: %s", body)
	}
	if !strings.Contains(body, `href="/proxy/x/dashboard"`) {
		t.Fatalf("root-relative link not prefixed: %s", body)
	}
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP should be dropped so the shim/assets load")
	}
	if !strings.Contains(body, "HTMLScriptElement.prototype") {
		t.Fatalf("shim does not rewrite runtime-injected asset URLs: %s", body)
	}
}

// When the upstream injects its own SourcePrefix (the API server proxy), those
// URLs map back to the public prefix and bare ones get prefixed once.
func TestServeMapsSourcePrefix(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><head></head><body><script src="/src/app.js"></script><a href="/page">p</a></body></html>`)
	}))
	defer upstream.Close()
	base, _ := url.Parse(upstream.URL)

	rec := httptest.NewRecorder()
	webproxy.Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x", SourcePrefix: "/src",
	})

	body := rec.Body.String()
	if !strings.Contains(body, `src="/proxy/x/app.js"`) {
		t.Fatalf("source prefix not mapped to public prefix: %s", body)
	}
	if strings.Contains(body, "/src/app.js") {
		t.Fatalf("source-prefixed URL leaked: %s", body)
	}
	if !strings.Contains(body, `href="/proxy/x/page"`) {
		t.Fatalf("bare root-relative link not prefixed: %s", body)
	}
}

func TestServeWorkerScope(t *testing.T) {
	rec := httptest.NewRecorder()
	webproxy.ServeWorker(rec, "/proxy/x")

	if got := rec.Header().Get("Service-Worker-Allowed"); got != "/proxy/x/" {
		t.Fatalf("worker scope header = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("worker content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/proxy/x") || !strings.Contains(body, `addEventListener("fetch"`) {
		t.Fatalf("worker body unexpected: %s", body)
	}
	// A rewritten request must carry its body, or POST/PUT lose their payload.
	if !strings.Contains(body, "init.body") || !strings.Contains(body, "r.blob()") {
		t.Fatalf("worker must forward the request body: %s", body)
	}
}
