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
	var base *url.URL
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		// A bare "/" home link, a deep link, and a form whose action is an absolute
		// URL built from the host we gave the app.
		_, _ = io.WriteString(w, `<html><head><title>x</title></head><body>`+
			`<a href="/">home</a><a href="/dashboard">d</a>`+
			`<form action="`+base.String()+`/web/login" method="post"></form></body></html>`)
	}))
	defer upstream.Close()
	base, _ = url.Parse(upstream.URL)

	rec := httptest.NewRecorder()
	webproxy.Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x",
	})

	body := rec.Body.String()
	if !strings.Contains(body, `href="/proxy/x/dashboard"`) {
		t.Fatalf("root-relative link not prefixed: %s", body)
	}
	if !strings.Contains(body, `href="/proxy/x/"`) {
		t.Fatalf("bare root link not prefixed: %s", body)
	}
	if !strings.Contains(body, `action="/proxy/x/web/login"`) {
		t.Fatalf("absolute upstream form action not mapped to prefix: %s", body)
	}
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP should be dropped so the shim/assets load")
	}
	if !strings.Contains(body, "HTMLScriptElement.prototype") {
		t.Fatalf("shim does not rewrite runtime-injected asset URLs: %s", body)
	}
	if !strings.Contains(body, "Location.prototype") {
		t.Fatalf("shim does not keep JS location navigations under the prefix: %s", body)
	}
}

// A redirect Location is mapped back under the prefix whether it is root-relative
// (incl. bare "/") or an absolute URL on the host the app was told to use.
func TestServeRewritesRedirectLocation(t *testing.T) {
	cases := map[string]string{
		"/web":               "/proxy/x/web",
		"/":                  "/proxy/x/",
		"UPSTREAM/web/login": "/proxy/x/web/login",
	}
	for loc, want := range cases {
		var base *url.URL
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			target := loc
			if strings.HasPrefix(loc, "UPSTREAM") {
				target = base.String() + strings.TrimPrefix(loc, "UPSTREAM")
			}
			w.Header().Set("Location", target)
			w.WriteHeader(http.StatusFound)
		}))
		base, _ = url.Parse(upstream.URL)

		rec := httptest.NewRecorder()
		webproxy.Serve(rec, httptest.NewRequest(http.MethodPost, "/", nil), webproxy.Options{
			Base: base, Transport: http.DefaultTransport, UpstreamPath: "/web/login", PublicPrefix: "/proxy/x",
		})
		if got := rec.Header().Get("Location"); got != want {
			t.Errorf("Location %q rewritten to %q, want %q", loc, got, want)
		}
		upstream.Close()
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

func TestServeRewritesCSSAndSrcset(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css")
			_, _ = io.WriteString(w, `a{background:url(/img/bg.png)}b{background:url("//cdn/x.png")}`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><head></head><body><img srcset="/a.png 1x, /b.png 2x"></body></html>`)
	}))
	defer upstream.Close()
	base, _ := url.Parse(upstream.URL)

	css := httptest.NewRecorder()
	webproxy.Serve(css, httptest.NewRequest(http.MethodGet, "/x.css", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/x.css", PublicPrefix: "/proxy/x",
	})
	if b := css.Body.String(); !strings.Contains(b, "url(/proxy/x/img/bg.png)") || !strings.Contains(b, `url("//cdn/x.png")`) {
		t.Fatalf("css url() rewrite wrong: %s", b)
	}

	html := httptest.NewRecorder()
	webproxy.Serve(html, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x",
	})
	if b := html.Body.String(); !strings.Contains(b, `srcset="/proxy/x/a.png 1x, /proxy/x/b.png 2x"`) {
		t.Fatalf("srcset rewrite wrong: %s", b)
	}
}

func TestServeRewritesSingleQuotedHTMLURLs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><head><meta http-equiv='refresh' content='0; url=/next'></head><body>`+
			`<a href='/'>home</a><script src='/app.js'></script><img srcset='/a.png 1x, /b.png 2x'></body></html>`)
	}))
	defer upstream.Close()
	base, _ := url.Parse(upstream.URL)

	rec := httptest.NewRecorder()
	webproxy.Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x",
	})

	body := rec.Body.String()
	for _, want := range []string{
		`href='/proxy/x/'`,
		`src='/proxy/x/app.js'`,
		`srcset='/proxy/x/a.png 1x, /proxy/x/b.png 2x'`,
		`content='0; url=/proxy/x/next'`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("single-quoted URL rewrite missing %q in %s", want, body)
		}
	}
}

func TestServeWorkerQuotesPrefixSafely(t *testing.T) {
	rec := httptest.NewRecorder()
	webproxy.ServeWorker(rec, `/proxy/"x\y`)

	body := rec.Body.String()
	if !strings.Contains(body, `var P="/proxy/\"x\\y"`) {
		t.Fatalf("worker prefix not safely quoted: %s", body)
	}
}

func TestServeRewritesCookiePath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Set-Cookie", "sid=abc; Path=/; HttpOnly")
		w.Header().Add("Set-Cookie", "__Host-sec=z; Path=/; Secure")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	base, _ := url.Parse(upstream.URL)

	rec := httptest.NewRecorder()
	webproxy.Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), webproxy.Options{
		Base: base, Transport: http.DefaultTransport, UpstreamPath: "/", PublicPrefix: "/proxy/x",
	})
	got := rec.Result().Header.Values("Set-Cookie")
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "sid=abc; Path=/proxy/x; HttpOnly") {
		t.Fatalf("normal cookie path not scoped: %q", got)
	}
	if !strings.Contains(joined, "__Host-sec=z; Path=/; Secure") {
		t.Fatalf("__Host- cookie path must stay /: %q", got)
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
