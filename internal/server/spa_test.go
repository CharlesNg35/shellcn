package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
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
