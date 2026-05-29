package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type nopSeekCloser struct{ *strings.Reader }

func (nopSeekCloser) Close() error { return nil }

func TestResolveRange(t *testing.T) {
	cases := []struct {
		header        string
		size          int64
		start, length int64
		status        int
	}{
		{"", 100, 0, 0, http.StatusOK},
		{"bytes=0-9", 100, 0, 10, http.StatusPartialContent},
		{"bytes=10-", 100, 10, 90, http.StatusPartialContent},
		{"bytes=-20", 100, 80, 20, http.StatusPartialContent},
		{"bytes=90-200", 100, 90, 10, http.StatusPartialContent}, // clamp end
		{"bytes=0-0", 100, 0, 1, http.StatusPartialContent},
		{"bytes=100-", 100, 0, 0, http.StatusRequestedRangeNotSatisfiable},
		{"bytes=5-4", 100, 0, 0, http.StatusRequestedRangeNotSatisfiable},
		{"bytes=0-9,20-29", 100, 0, 0, http.StatusOK}, // multi-range -> full
		{"items=0-9", 100, 0, 0, http.StatusOK},       // malformed -> full
		{"bytes=-10", 0, 0, 0, http.StatusRequestedRangeNotSatisfiable},
	}
	for _, c := range cases {
		start, length, status := resolveRange(c.header, c.size)
		if start != c.start || length != c.length || status != c.status {
			t.Errorf("resolveRange(%q,%d) = (%d,%d,%d), want (%d,%d,%d)",
				c.header, c.size, start, length, status, c.start, c.length, c.status)
		}
	}
}

func serveDownload(t *testing.T, method, rangeHdr string, dl *plugin.Download) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/x", nil)
	if rangeHdr != "" {
		r.Header.Set("Range", rangeHdr)
	}
	rec := httptest.NewRecorder()
	(&Server{}).writeDownload(rec, r, dl)
	return rec
}

func TestWriteDownloadSeekerRange(t *testing.T) {
	data := "0123456789"
	rec := serveDownload(t, http.MethodGet, "bytes=2-5", &plugin.Download{
		Name: "f.bin", MIME: "application/octet-stream", Size: int64(len(data)),
		Seeker: nopSeekCloser{strings.NewReader(data)},
	})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if rec.Body.String() != "2345" {
		t.Fatalf("body = %q, want 2345", rec.Body.String())
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
}

func TestWriteDownloadOpenRange(t *testing.T) {
	data := "abcdefghij"
	open := func(off, length int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(data[off : off+length])), nil
	}
	mk := func() *plugin.Download {
		return &plugin.Download{Name: "f", MIME: "text/plain", Size: int64(len(data)), OpenRange: open}
	}

	rec := serveDownload(t, http.MethodGet, "bytes=3-6", mk())
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "defg" {
		t.Fatalf("range: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 3-6/10" {
		t.Fatalf("Content-Range = %q", got)
	}

	rec = serveDownload(t, http.MethodGet, "", mk())
	if rec.Code != http.StatusOK || rec.Body.String() != data {
		t.Fatalf("full: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatal("missing Accept-Ranges")
	}

	rec = serveDownload(t, http.MethodGet, "bytes=50-", mk())
	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("unsatisfiable: status=%d", rec.Code)
	}
}

func TestWriteDownloadBodyAndHead(t *testing.T) {
	rec := serveDownload(t, http.MethodGet, "", &plugin.Download{
		Name: "a.txt", MIME: "text/plain", Size: 5, Body: io.NopCloser(strings.NewReader("hello")),
	})
	if rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("body: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = serveDownload(t, http.MethodHead, "", &plugin.Download{
		Name: "a.txt", MIME: "text/plain", Size: 5, Body: io.NopCloser(strings.NewReader("hello")),
	})
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("head: status=%d bodyLen=%d", rec.Code, rec.Body.Len())
	}
}

func TestWriteDownloadInline(t *testing.T) {
	rec := serveDownload(t, http.MethodGet, "", &plugin.Download{
		Name: "i.png", MIME: "image/png", Size: 2, Inline: true,
		Body: io.NopCloser(strings.NewReader("hi")),
	})
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Fatalf("Content-Disposition = %q, want inline", cd)
	}
	if rec.Header().Get("Content-Security-Policy") != "sandbox" {
		t.Fatal("inline must set CSP sandbox")
	}
}
