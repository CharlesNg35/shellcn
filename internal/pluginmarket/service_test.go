package pluginmarket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/charlesng35/shellcn/sdk/grpcplugin"
)

func testIndex(assetURL, sha string) Index {
	return Index{
		SchemaVersion: 1,
		Plugins: []Entry{{
			Name: "demo", DisplayName: "Demo", Description: "d", Repo: "github.com/a/b",
			License: "MIT", Maintainers: []string{"a"},
			Versions: []Version{{
				Version: "0.2.0", SDK: "v0.1.3", APIVersion: 1, ProtocolVersion: grpcplugin.ProtocolVersion,
				Assets: map[string]Asset{
					runtime.GOOS + "/" + runtime.GOARCH: {SHA256: sha, URLs: []string{assetURL}},
				},
			}},
		}},
	}
}

func serveIndex(t *testing.T, idx *Index, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("If-None-Match") == "v1" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", "v1")
		_ = json.NewEncoder(w).Encode(idx)
	}))
}

func TestEntriesCachesAndRevalidates(t *testing.T) {
	idx := testIndex("https://example.invalid/x", "00")
	var hits atomic.Int64
	srv := serveIndex(t, &idx, &hits)
	t.Cleanup(srv.Close)

	s := New([]string{srv.URL})
	ctx := context.Background()
	for range 3 {
		entries, err := s.Entries(ctx)
		if err != nil {
			t.Fatalf("entries: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "demo" {
			t.Fatalf("entries: %+v", entries)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("TTL cache should make one request, got %d", hits.Load())
	}

	s.fetched = s.fetched.Add(-2 * refreshAfter)
	if _, err := s.Entries(ctx); err != nil {
		t.Fatalf("revalidate: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected conditional revalidation, got %d hits", hits.Load())
	}
}

func TestEntriesServesStaleOnFailure(t *testing.T) {
	idx := testIndex("https://example.invalid/x", "00")
	var hits atomic.Int64
	srv := serveIndex(t, &idx, &hits)

	s := New([]string{srv.URL})
	if _, err := s.Entries(context.Background()); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	srv.Close()
	s.fetched = s.fetched.Add(-2 * refreshAfter)
	entries, err := s.Entries(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("stale serve failed: %v %v", entries, err)
	}
}

func TestMergeFirstURLWins(t *testing.T) {
	a := testIndex("https://example.invalid/a", "aa")
	a.Plugins[0].Description = "from-a"
	b := testIndex("https://example.invalid/b", "bb")
	b.Plugins[0].Description = "from-b"
	b.Plugins = append(b.Plugins, Entry{Name: "extra", Versions: []Version{}})

	var hits atomic.Int64
	srvA, srvB := serveIndex(t, &a, &hits), serveIndex(t, &b, &hits)
	t.Cleanup(srvA.Close)
	t.Cleanup(srvB.Close)

	entries, err := New([]string{srvA.URL, srvB.URL}).Entries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Description != "from-a" {
		t.Fatalf("merge: %+v", entries)
	}
}

func TestInstallVerifiesAndFallsBack(t *testing.T) {
	payload := []byte("plugin-binary-bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(dead.Close)
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(good.Close)

	platform := runtime.GOOS + "/" + runtime.GOARCH
	e := Entry{Name: "demo"}
	v := Version{Assets: map[string]Asset{platform: {SHA256: sha, URLs: []string{dead.URL, good.URL}}}}

	dir := t.TempDir()
	path, err := New(nil).Install(context.Background(), e, v, dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if path != filepath.Join(dir, "demo") {
		t.Fatalf("dest: %s", path)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(payload) {
		t.Fatal("payload mismatch")
	}
	if info, _ := os.Stat(path); info.Mode()&0o111 == 0 {
		t.Fatal("installed binary must be executable")
	}
}

func TestInstallRejectsTamperedBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "tampered")
	}))
	t.Cleanup(srv.Close)

	platform := runtime.GOOS + "/" + runtime.GOARCH
	v := Version{Assets: map[string]Asset{platform: {
		SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
		URLs:   []string{srv.URL},
	}}}
	dir := t.TempDir()
	if _, err := New(nil).Install(context.Background(), Entry{Name: "demo"}, v, dir); err == nil {
		t.Fatal("tampered download must be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); !os.IsNotExist(err) {
		t.Fatal("nothing may be installed on verification failure")
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".demo.download-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temp files must be cleaned up: %v", leftovers)
	}
}

func TestInstallableFilters(t *testing.T) {
	platform := runtime.GOOS + "/" + runtime.GOARCH
	e := Entry{Versions: []Version{
		{Version: "0.4.0", ProtocolVersion: grpcplugin.ProtocolVersion + 1, Assets: map[string]Asset{platform: {}}},
		{Version: "0.3.0", ProtocolVersion: grpcplugin.ProtocolVersion, Yanked: true, Assets: map[string]Asset{platform: {}}},
		{Version: "0.2.0", ProtocolVersion: grpcplugin.ProtocolVersion, Assets: map[string]Asset{"plan9/mips": {}}},
		{Version: "0.1.0", ProtocolVersion: grpcplugin.ProtocolVersion, Assets: map[string]Asset{platform: {}}},
	}}
	v, ok := Installable(e)
	if !ok || v.Version != "0.1.0" {
		t.Fatalf("installable = %v %v, want 0.1.0", v.Version, ok)
	}
	if _, err := FindVersion(e, "0.3.0"); err == nil {
		t.Fatal("yanked version must not resolve")
	}
	if _, err := FindVersion(e, "0.4.0"); err == nil {
		t.Fatal("wire-incompatible version must not resolve")
	}
}
