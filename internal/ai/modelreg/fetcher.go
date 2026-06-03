package modelreg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// providerModelsTTL caches a provider's model list; it changes rarely.
const providerModelsTTL = 2 * time.Hour

// defaultBaseURLs are the API endpoints for the built-in vendors. An
// openai_compatible provider must supply its own base URL.
var defaultBaseURLs = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"anthropic":  "https://api.anthropic.com/v1",
	"google":     "https://generativelanguage.googleapis.com/v1beta",
}

// nonChatPattern excludes models that are not chat-completion capable.
var nonChatPattern = regexp.MustCompile(`(?i)\b(embed|embedding|tts|whisper|dall-e|stable-diffusion|rerank|moderation)\b`)

// ProviderModel is a chat model advertised by a provider's API.
type ProviderModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ContextWindow   int    `json:"contextWindow,omitempty"`
	MaxOutputTokens int    `json:"maxOutputTokens,omitempty"`
}

// FetchModels lists a provider's chat models from its API, cached per
// provider+baseURL. kind selects the wire format; baseURL falls back to the
// vendor default for built-in kinds.
func (r *Registry) FetchModels(ctx context.Context, kind, baseURL, apiKey string) ([]ProviderModel, error) {
	base := r.resolveBaseURL(kind, baseURL)
	if base == "" {
		return nil, &httpError{status: http.StatusBadRequest}
	}
	cache := r.providerCache(kind, base, apiKey)
	models, err := cache.getOrFetch(func() ([]ProviderModel, error) {
		return r.fetchModels(ctx, kind, base, apiKey)
	})
	if err != nil {
		return nil, err
	}
	r.cacheProviderModelLimits(kind, models)
	return models, nil
}

func (r *Registry) fetchModels(ctx context.Context, kind, base, apiKey string) ([]ProviderModel, error) {
	switch kind {
	case "google":
		return r.fetchGoogle(ctx, base, apiKey)
	case "anthropic":
		return r.fetchAnthropic(ctx, base, apiKey)
	default: // openai + openai_compatible
		return r.fetchOpenAICompatible(ctx, base, apiKey)
	}
}

func (r *Registry) fetchGoogle(ctx context.Context, base, apiKey string) ([]ProviderModel, error) {
	var body struct {
		Models []struct {
			Name            string `json:"name"`
			DisplayName     string `json:"displayName"`
			InputTokenLimit int    `json:"inputTokenLimit"`
		} `json:"models"`
	}
	u := base + "/models"
	if apiKey != "" {
		u += "?key=" + url.QueryEscape(apiKey)
	}
	if err := r.getJSON(ctx, u, &body); err != nil {
		return nil, err
	}
	var out []ProviderModel
	for _, m := range body.Models {
		if !strings.HasPrefix(m.Name, "models/gemini") || nonChatPattern.MatchString(m.Name) {
			continue
		}
		out = append(out, ProviderModel{
			ID:            strings.TrimPrefix(m.Name, "models/"),
			Name:          m.DisplayName,
			ContextWindow: posInt(m.InputTokenLimit),
		})
	}
	return sortModels(out), nil
}

func (r *Registry) fetchAnthropic(ctx context.Context, base, apiKey string) ([]ProviderModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	var body struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			MaxInput    int    `json:"max_input_tokens"`
		} `json:"data"`
	}
	if err := r.doJSON(req, &body); err != nil {
		return nil, err
	}
	var out []ProviderModel
	for _, m := range body.Data {
		if nonChatPattern.MatchString(m.ID) {
			continue
		}
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		out = append(out, ProviderModel{ID: m.ID, Name: name, ContextWindow: posInt(m.MaxInput)})
	}
	return sortModels(out), nil
}

func (r *Registry) fetchOpenAICompatible(ctx context.Context, base, apiKey string) ([]ProviderModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	var body struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
			TopProvider   struct {
				ContextLength      int `json:"context_length"`
				MaxCompletionToken int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if err := r.doJSON(req, &body); err != nil {
		return nil, err
	}
	var out []ProviderModel
	for _, m := range body.Data {
		if nonChatPattern.MatchString(m.ID) {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		cw := m.ContextLength
		if cw <= 0 {
			cw = m.TopProvider.ContextLength
		}
		out = append(out, ProviderModel{
			ID:              m.ID,
			Name:            name,
			ContextWindow:   posInt(cw),
			MaxOutputTokens: posInt(m.TopProvider.MaxCompletionToken),
		})
	}
	return sortModels(out), nil
}

func (r *Registry) resolveBaseURL(kind, baseURL string) string {
	if b := strings.TrimRight(strings.TrimSpace(baseURL), "/"); b != "" {
		return b
	}
	return defaultBaseURLs[kind]
}

func (r *Registry) providerCache(kind, base, apiKey string) *ttlCache[[]ProviderModel] {
	key := kind + "|" + base + "|" + cacheKeySecret(apiKey)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.providerCaches == nil {
		r.providerCaches = map[string]*ttlCache[[]ProviderModel]{}
	}
	c, ok := r.providerCaches[key]
	if !ok {
		c = newTTLCache[[]ProviderModel](providerModelsTTL, nil)
		r.providerCaches[key] = c
	}
	return c
}

func cacheKeySecret(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func (r *Registry) cacheProviderModelLimits(kind string, models []ProviderModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range models {
		l, ok := mergeLimits(Limits{ContextWindow: m.ContextWindow, MaxOutputTokens: m.MaxOutputTokens})
		if !ok {
			continue
		}
		for _, key := range []string{kind + "|" + m.ID, "|" + m.ID} {
			r.resolved[strings.ToLower(key)] = l
		}
	}
}

func sortModels(m []ProviderModel) []ProviderModel {
	sort.Slice(m, func(i, j int) bool { return m[i].ID < m[j].ID })
	return m
}
