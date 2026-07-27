package version

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsDev(t *testing.T) {
	cases := map[string]bool{
		"":            true,
		"dev":         true,
		"v0.1.0-dev":  true,
		"v1.2.3-rc1":  true,
		"1.2.3":       true, // missing the leading v -> not a valid tag here
		"v1.2.3+meta": true,
		"v0.1.0":      false,
		"v1.2.3":      false,
		"v10.20.30":   false,
	}
	for v, want := range cases {
		if got := IsDev(v); got != want {
			t.Errorf("IsDev(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestCheckDevSkipsNetwork(t *testing.T) {
	c := NewChecker("v0.1.0-dev", true)
	info := c.Check(context.Background(), true)
	if !info.Dev || info.UpdateAvailable || info.CheckedAt != nil {
		t.Fatalf("dev build must not check: %+v", info)
	}
}

func TestCheckDisabled(t *testing.T) {
	c := NewChecker("v0.1.0", false)
	info := c.Check(context.Background(), true)
	if !info.CheckDisabled || info.UpdateAvailable || info.CheckedAt != nil {
		t.Fatalf("disabled checker must not check: %+v", info)
	}
}

func TestCheckDetectsUpdateAndCaches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.test/releases/v0.2.0"}`))
	}))
	defer srv.Close()

	c := NewChecker("v0.1.0", true)
	c.overrideEndpoint(srv.URL)

	info := c.Check(context.Background(), false)
	if !info.UpdateAvailable || info.Latest != "v0.2.0" {
		t.Fatalf("expected update to v0.2.0, got %+v", info)
	}
	if info.ReleaseURL != "https://example.test/releases/v0.2.0" {
		t.Fatalf("release url = %q", info.ReleaseURL)
	}
	// Second call within the TTL must be served from cache.
	if c.Check(context.Background(), false); calls != 1 {
		t.Fatalf("expected 1 upstream call (cached), got %d", calls)
	}
}

func TestCheckNoUpdateWhenCurrentIsLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer srv.Close()

	c := NewChecker("v0.2.0", true)
	c.overrideEndpoint(srv.URL)
	if info := c.Check(context.Background(), false); info.UpdateAvailable {
		t.Fatalf("no update expected when running the latest: %+v", info)
	}
}

func TestCheckUpstreamErrorIsSoft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewChecker("v0.1.0", true)
	c.overrideEndpoint(srv.URL)
	info := c.Check(context.Background(), false)
	if info.Error == "" || info.UpdateAvailable {
		t.Fatalf("expected a soft error, got %+v", info)
	}
}
