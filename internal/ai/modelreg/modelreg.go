// Package modelreg resolves model token limits from live registries, provider
// catalogues, static metadata, and finally a safe default.
package modelreg

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	openRouterURL = "https://openrouter.ai/api/v1/models"
	modelsDevURL  = "https://models.dev/api.json"
	cacheTTL      = 24 * time.Hour
	fetchTimeout  = 10 * time.Second
	defaultWindow = 128_000
)

// Limits are a model's resolved token limits (0 = unknown).
type Limits struct {
	ContextWindow   int
	MaxOutputTokens int
}

// Registry resolves model limits. It is safe for concurrent use.
type Registry struct {
	http   *http.Client
	logger *slog.Logger
	orURL  string
	mdURL  string

	or *ttlCache[[]openRouterModel]
	md *ttlCache[map[string]Limits]

	mu             sync.Mutex
	resolved       map[string]Limits
	providerCaches map[string]*ttlCache[[]ProviderModel]
}

// Option configures a Registry.
type Option func(*Registry)

// WithHTTPClient overrides the HTTP client (used in tests).
func WithHTTPClient(c *http.Client) Option { return func(r *Registry) { r.http = c } }

// WithURLs overrides the registry endpoints (used in tests).
func WithURLs(openRouter, modelsDev string) Option {
	return func(r *Registry) { r.orURL, r.mdURL = openRouter, modelsDev }
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option { return func(r *Registry) { r.logger = l } }

// New builds a registry.
func New(opts ...Option) *Registry {
	r := &Registry{
		http:     &http.Client{Timeout: fetchTimeout},
		logger:   slog.Default(),
		orURL:    openRouterURL,
		mdURL:    modelsDevURL,
		or:       newTTLCache[[]openRouterModel](cacheTTL, nil),
		md:       newTTLCache[map[string]Limits](cacheTTL, nil),
		resolved: map[string]Limits{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ContextWindow resolves a model's context window from the registries, or the
// safe default when unknown. Never returns 0.
func (r *Registry) ContextWindow(ctx context.Context, modelID, providerID string) int {
	if l, ok := r.Lookup(ctx, modelID, providerID); ok && l.ContextWindow > 0 {
		return l.ContextWindow
	}
	return defaultWindow
}

// Lookup resolves a model's limits across the registries (cached), preferring a
// previously memoized result.
func (r *Registry) Lookup(ctx context.Context, modelID, providerID string) (Limits, bool) {
	key := strings.ToLower(providerID + "|" + modelID)
	r.mu.Lock()
	if l, ok := r.resolved[key]; ok {
		r.mu.Unlock()
		return l, true
	}
	r.mu.Unlock()

	r.ensureRegistries(ctx)

	modelCandidates := buildCandidates(modelID)
	candidates := append(buildProviderCandidates(providerID, modelCandidates), modelCandidates...)
	allowPrefix := strings.Contains(modelID, ":")

	var matches []Limits
	if orModels, ok := staleOr(r.or); ok {
		if l, found := findInModels(candidates, orModels, allowPrefix); found {
			matches = append(matches, l)
		}
	}
	if mdReg, ok := staleOr(r.md); ok {
		if l, found := findInRegistry(candidates, mdReg, allowPrefix); found {
			matches = append(matches, l)
		}
	}

	merged, ok := mergeLimits(matches...)
	if ok {
		r.mu.Lock()
		r.resolved[key] = merged
		r.mu.Unlock()
	}
	return merged, ok
}

// staleOr returns the cache's fresh value, else its stale value.
func staleOr[T any](c *ttlCache[T]) (T, bool) {
	if v, ok := c.get(); ok {
		return v, true
	}
	return c.getStale()
}

func (r *Registry) ensureRegistries(ctx context.Context) {
	var wg sync.WaitGroup
	if r.orURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.or.getOrFetch(func() ([]openRouterModel, error) { return r.fetchOpenRouter(ctx) }); err != nil {
				r.logger.Warn("model registry fetch failed", "source", "openrouter", "err", err)
			}
		}()
	}
	if r.mdURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := r.md.getOrFetch(func() (map[string]Limits, error) { return r.fetchModelsDev(ctx) }); err != nil {
				r.logger.Warn("model registry fetch failed", "source", "models.dev", "err", err)
			}
		}()
	}
	wg.Wait()
}

// ── OpenRouter ──

type openRouterModel struct {
	ID            string `json:"id"`
	ContextLength int    `json:"context_length"`
	TopProvider   struct {
		ContextLength      int `json:"context_length"`
		MaxCompletionToken int `json:"max_completion_tokens"`
	} `json:"top_provider"`
}

func (m openRouterModel) limits() (Limits, bool) {
	cw := m.ContextLength
	if cw <= 0 {
		cw = m.TopProvider.ContextLength
	}
	return mergeLimits(Limits{ContextWindow: posInt(cw), MaxOutputTokens: posInt(m.TopProvider.MaxCompletionToken)})
}

func (r *Registry) fetchOpenRouter(ctx context.Context) ([]openRouterModel, error) {
	var body struct {
		Data []openRouterModel `json:"data"`
	}
	if err := r.getJSON(ctx, r.orURL, &body); err != nil {
		return nil, err
	}
	return body.Data, nil
}

func findInModels(candidates []string, models []openRouterModel, allowPrefix bool) (Limits, bool) {
	for _, c := range candidates { // exact id
		for _, m := range models {
			if l, ok := m.limits(); ok && strings.ToLower(m.ID) == c {
				return l, true
			}
		}
	}
	for _, c := range candidates { // base name
		for _, m := range models {
			if l, ok := m.limits(); ok && extractBaseName(m.ID) == c {
				return l, true
			}
		}
	}
	if allowPrefix {
		for _, c := range candidates {
			for _, m := range models {
				l, ok := m.limits()
				if !ok {
					continue
				}
				id := strings.ToLower(m.ID)
				if strings.Contains(c, "/") && isMatch(id, c) {
					return l, true
				}
				if !strings.Contains(c, "/") && isMatch(extractBaseName(m.ID), c) {
					return l, true
				}
			}
		}
	}
	return Limits{}, false
}

// ── models.dev ──

func (r *Registry) fetchModelsDev(ctx context.Context) (map[string]Limits, error) {
	var raw map[string]struct {
		ID     string `json:"id"`
		Models map[string]struct {
			ID    string `json:"id"`
			Limit struct {
				Context int `json:"context"`
				Output  int `json:"output"`
			} `json:"limit"`
		} `json:"models"`
	}
	if err := r.getJSON(ctx, r.mdURL, &raw); err != nil {
		return nil, err
	}
	out := map[string]Limits{}
	for providerKey, provider := range raw {
		for modelKey, model := range provider.Models {
			cw, mo := posInt(model.Limit.Context), posInt(model.Limit.Output)
			if cw == 0 && mo == 0 {
				continue
			}
			l := Limits{ContextWindow: cw, MaxOutputTokens: mo}
			for _, k := range []string{model.ID, modelKey, provider.ID + "/" + model.ID, providerKey + "/" + model.ID} {
				if k != "" {
					out[strings.ToLower(k)] = l
				}
			}
		}
	}
	return out, nil
}

func findInRegistry(candidates []string, registry map[string]Limits, allowPrefix bool) (Limits, bool) {
	for _, c := range candidates {
		if l, ok := registry[c]; ok {
			return l, true
		}
	}
	for _, c := range candidates {
		for k, l := range registry {
			if extractBaseName(k) == c {
				return l, true
			}
		}
	}
	if allowPrefix {
		for _, c := range candidates {
			for k, l := range registry {
				if strings.Contains(c, "/") && isMatch(strings.ToLower(k), c) {
					return l, true
				}
				if !strings.Contains(c, "/") && isMatch(extractBaseName(k), c) {
					return l, true
				}
			}
		}
	}
	return Limits{}, false
}

// ── HTTP ──

func (r *Registry) getJSON(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return r.doJSON(req, dst)
}

func (r *Registry) doJSON(req *http.Request, dst any) error {
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return &httpError{status: resp.StatusCode}
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(dst)
}

type httpError struct{ status int }

func (e *httpError) Error() string { return "model registry: HTTP " + http.StatusText(e.status) }

// ProviderHTTPStatus exposes upstream provider/catalogue HTTP failures to
// service boundaries without leaking transport implementation details.
func ProviderHTTPStatus(err error) (int, bool) {
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		return httpErr.status, true
	}
	return 0, false
}

func posInt(v int) int {
	if v > 0 {
		return v
	}
	return 0
}
