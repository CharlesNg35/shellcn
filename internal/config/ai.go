package config

import "strings"

// AIConfig is the optional operator-provided shared AI configuration. It is
// loaded from config.yaml + SHELLCN_AI_* env and exposed to clients only as a
// non-secret status projection.
type AIConfig struct {
	Kind    string `mapstructure:"kind"`
	Name    string `mapstructure:"name"`
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
}

// Configured reports whether a usable shared AI provider is present.
func (c AIConfig) Configured() bool {
	if strings.TrimSpace(c.APIKey) == "" ||
		strings.TrimSpace(c.Kind) == "" ||
		strings.TrimSpace(c.Model) == "" {
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
