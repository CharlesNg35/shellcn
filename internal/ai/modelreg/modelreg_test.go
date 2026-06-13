package modelreg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildCandidatesOrdersMostSpecificFirst(t *testing.T) {
	c := buildCandidates("openai/gpt-5-mini")
	if len(c) == 0 || c[0] != "openai/gpt-5-mini" {
		t.Fatalf("full id should come first: %v", c)
	}
	if !contains(c, "gpt-5-mini") {
		t.Fatalf("base name missing: %v", c)
	}

	// A tagged ollama-style id expands quant/param variants.
	tagged := buildCandidates("granite4.1:8b")
	if !contains(tagged, "granite-4.1:8b") && !contains(tagged, "granite4.1:8b") {
		t.Fatalf("expected separator variants: %v", tagged)
	}
}

func TestIsMatchBoundary(t *testing.T) {
	if !isMatch("gpt-4o-mini", "gpt-4o") {
		t.Fatal("prefix on a dash boundary should match")
	}
	if isMatch("gpt-4omini", "gpt-4o") {
		t.Fatal("prefix without a boundary must not match")
	}
}

func TestLookupResolvesFromRegistries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/openrouter":
			_, _ = w.Write([]byte(`{"data":[{"id":"openai/gpt-4o","context_length":128000,"top_provider":{"max_completion_tokens":16384}}]}`))
		case "/modelsdev":
			_, _ = w.Write([]byte(`{"openai":{"id":"openai","models":{"gpt-4o":{"id":"gpt-4o","limit":{"context":128000,"output":16384}}}}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	r := New(WithURLs(srv.URL+"/openrouter", srv.URL+"/modelsdev"), WithHTTPClient(srv.Client()))
	lim, ok := r.Lookup(context.Background(), "gpt-4o", "openai")
	if !ok || lim.ContextWindow != 128000 || lim.MaxOutputTokens != 16384 {
		t.Fatalf("lookup wrong: %+v ok=%v", lim, ok)
	}
	if cw := r.ContextWindow(context.Background(), "gpt-4o", "openai"); cw != 128000 {
		t.Fatalf("context window = %d", cw)
	}
}

func TestContextWindowFallsBackToDefaultWithoutMetadata(t *testing.T) {
	r := New(WithoutRegistryFetch())
	if cw := r.ContextWindow(context.Background(), "some-unknown-model", "openai"); cw != defaultWindow {
		t.Fatalf("no registry metadata should fall back to default, got %d", cw)
	}
}

func TestFetchModelsOpenAICompatible(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"gpt-4o","context_length":128000,"top_provider":{"max_completion_tokens":16384}},
			{"id":"text-embedding-3-small"}
		]}`))
	}))
	defer srv.Close()

	r := New(WithHTTPClient(srv.Client()))
	models, err := r.FetchModels(context.Background(), "openai_compatible", srv.URL, "sk-test")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("auth header not sent: %q", gotAuth)
	}
	// The embedding model is filtered out as non-chat.
	if len(models) != 1 || models[0].ID != "gpt-4o" || models[0].ContextWindow != 128000 {
		t.Fatalf("unexpected models: %+v", models)
	}
	if lim, ok := r.Lookup(context.Background(), "gpt-4o", "openai_compatible"); !ok || lim.ContextWindow != 128000 || lim.MaxOutputTokens != 16384 {
		t.Fatalf("fetched model limits were not cached: %+v ok=%v", lim, ok)
	}
}

func TestFetchModelsCacheIsScopedToAPIKey(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("Authorization") {
		case "Bearer sk-a":
			_, _ = w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
		case "Bearer sk-b":
			_, _ = w.Write([]byte(`{"data":[{"id":"model-b"}]}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()

	r := New(WithHTTPClient(srv.Client()))
	a, err := r.FetchModels(context.Background(), "openai_compatible", srv.URL, "sk-a")
	if err != nil {
		t.Fatalf("fetch a: %v", err)
	}
	b, err := r.FetchModels(context.Background(), "openai_compatible", srv.URL, "sk-b")
	if err != nil {
		t.Fatalf("fetch b: %v", err)
	}
	if calls != 2 {
		t.Fatalf("model cache should be credential-scoped, calls=%d", calls)
	}
	if len(a) != 1 || a[0].ID != "model-a" || len(b) != 1 || b[0].ID != "model-b" {
		t.Fatalf("unexpected model lists: a=%+v b=%+v", a, b)
	}
}

func TestFetchModelsGoogleFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[
			{"name":"models/gemini-2.5-pro","displayName":"Gemini 2.5 Pro","inputTokenLimit":1000000},
			{"name":"models/embedding-001","displayName":"Embedding"}
		]}`))
	}))
	defer srv.Close()

	r := New(WithHTTPClient(srv.Client()))
	models, err := r.FetchModels(context.Background(), "google", srv.URL, "key")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gemini-2.5-pro" || models[0].ContextWindow != 1000000 {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestProviderHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	r := New(WithHTTPClient(srv.Client()))
	if _, err := r.FetchModels(context.Background(), "anthropic", srv.URL, ""); err == nil {
		t.Fatal("fetch should fail")
	} else if status, ok := ProviderHTTPStatus(err); !ok || status != http.StatusUnauthorized {
		t.Fatalf("status = %d ok=%v err=%v", status, ok, err)
	}
}

func TestResolveBaseURLPerKind(t *testing.T) {
	r := New(WithoutRegistryFetch())
	for _, kind := range []string{"openai", "openrouter", "anthropic", "google"} {
		if r.resolveBaseURL(kind, "") == "" {
			t.Errorf("vendor kind %q has no default base URL", kind)
		}
	}
	// openai_compatible has no default; it requires an explicit base URL.
	if r.resolveBaseURL("openai_compatible", "") != "" {
		t.Error("openai_compatible should have no default base URL")
	}
	if r.resolveBaseURL("openai_compatible", "http://host/v1/") != "http://host/v1" {
		t.Error("explicit base URL should be normalized and used")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
