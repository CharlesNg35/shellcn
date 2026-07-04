package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"testing/fstest"

	"github.com/charlesng35/shellcn/sdk/plugin/webproxy"
)

func TestSPAHandlerFallsBackForDirectories(t *testing.T) {
	s := &Server{deps: Deps{StaticFS: fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte("spa shell")},
		"assets/app.js":   &fstest.MapFile{Data: []byte("asset")},
		"assets/nested":   &fstest.MapFile{Mode: 0o755 | fs.ModeDir},
		"assets/child.js": &fstest.MapFile{Data: []byte("child")},
	}}}

	for _, path := range []string{"/login", "/assets", "/assets/nested"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		s.spaHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, rec.Code)
		}
		if rec.Body.String() != "spa shell" {
			t.Fatalf("%s: body = %q, want SPA shell", path, rec.Body.String())
		}
	}
}

func TestSPAHandlerServesRealAssets(t *testing.T) {
	s := &Server{deps: Deps{StaticFS: fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("spa shell")},
		"assets/app.js": &fstest.MapFile{Data: []byte("asset")},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()

	s.spaHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "asset" {
		t.Fatalf("body = %q, want asset", rec.Body.String())
	}
}

func TestSPAHandlerRedirectsEscapedWebProxyNavigation(t *testing.T) {
	s := &Server{deps: Deps{StaticFS: fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("spa shell")},
	}}}
	prefix := "/api/connections/ffe78488-e333-468c-9f36-ce65c601947d/proxy/services/dolibarr/web/80"
	req := httptest.NewRequest(http.MethodGet, "https://shell.smatflow.xyz/admin/company.php?mainmenu=home&action=edit", nil)
	req.Header.Set("Referer", "https://shell.smatflow.xyz"+prefix+"/admin/index.php?mainmenu=home")
	req.AddCookie(&http.Cookie{Name: webproxy.PrefixCookieName, Value: url.QueryEscape(prefix)})
	rec := httptest.NewRecorder()

	s.spaHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	want := prefix + "/admin/company.php?mainmenu=home&action=edit"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestSPAHandlerSelectsMatchingWebProxyPrefixFromCookie(t *testing.T) {
	s := &Server{deps: Deps{StaticFS: fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("spa shell")},
	}}}
	latest := "/api/connections/newer/proxy/services/default/other/80"
	prefix := "/api/connections/ffe78488-e333-468c-9f36-ce65c601947d/proxy/services/dolibarr/web/80"
	req := httptest.NewRequest(http.MethodGet, "https://shell.smatflow.xyz/admin/company.php?mainmenu=home", nil)
	req.Header.Set("Referer", "https://shell.smatflow.xyz"+prefix+"/admin/index.php")
	req.AddCookie(&http.Cookie{Name: webproxy.PrefixCookieName, Value: url.QueryEscape(latest + "\n" + prefix)})
	rec := httptest.NewRecorder()

	s.spaHandler().ServeHTTP(rec, req)

	want := prefix + "/admin/company.php?mainmenu=home"
	if got := rec.Header().Get("Location"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestSPAHandlerDoesNotRedirectUnrelatedProxyEscape(t *testing.T) {
	s := &Server{deps: Deps{StaticFS: fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("spa shell")},
	}}}
	prefix := "/api/connections/c1/proxy/services/default/app/80"
	req := httptest.NewRequest(http.MethodGet, "https://shell.smatflow.xyz/admin/company.php", nil)
	req.Header.Set("Referer", "https://shell.smatflow.xyz/settings")
	req.AddCookie(&http.Cookie{Name: webproxy.PrefixCookieName, Value: url.QueryEscape(prefix)})
	rec := httptest.NewRecorder()

	s.spaHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want SPA fallback 200", rec.Code)
	}
	if rec.Body.String() != "spa shell" {
		t.Fatalf("body = %q, want SPA shell", rec.Body.String())
	}
}
