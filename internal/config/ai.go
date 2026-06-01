package config

import "strings"

// AIConfig is the optional operator-provided "shared AI" configuration. It is an
// infrastructure decision (vendor, cost, key custody), so it lives with the
// other bootstrap settings — loaded from config.yaml + SHELLCN_AI_* env — and
// never touches the database or a runtime admin UI. The API key stays in env /
// secret-manager and is never returned to clients; only a read-only projection
// (presence + provider/model) is exposed. The model is pinned: users see which
// model was used but cannot switch the shared config.
type AIConfig struct {
	Kind         string   `mapstructure:"kind"`          // openai | openrouter | anthropic | google | openai_compatible
	Name         string   `mapstructure:"name"`          // display name shown as "Shared AI"
	BaseURL      string   `mapstructure:"base_url"`      // required for openai_compatible endpoints
	APIKey       string   `mapstructure:"api_key"`       // env-preferred (SHELLCN_AI_API_KEY); never persisted
	Models       []string `mapstructure:"models"`        // optional allow-list
	DefaultModel string   `mapstructure:"default_model"` // the pinned model
}

// Configured reports whether a usable shared AI provider is present. A key, a
// kind, and a default model are the minimum; openai_compatible also needs a base
// URL since it has no implicit endpoint.
func (c AIConfig) Configured() bool {
	if strings.TrimSpace(c.APIKey) == "" ||
		strings.TrimSpace(c.Kind) == "" ||
		strings.TrimSpace(c.DefaultModel) == "" {
		return false
	}
	if c.Kind == "openai_compatible" && strings.TrimSpace(c.BaseURL) == "" {
		return false
	}
	return true
}

// DisplayName is the name shown to users, falling back to the kind.
func (c AIConfig) DisplayName() string {
	if n := strings.TrimSpace(c.Name); n != "" {
		return n
	}
	return c.Kind
}
