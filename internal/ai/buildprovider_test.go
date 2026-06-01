package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
)

// Every built-in kind must map to an engine adapter; only an unknown kind is
// unsupported. (Construction may fail for other reasons offline — we only assert
// the kind is recognized.)
func TestBuildProviderCoversEveryKind(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []models.AIProviderKind{
		models.AIProviderOpenAI,
		models.AIProviderAnthropic,
		models.AIProviderGoogle,
		models.AIProviderOpenAICompat,
	} {
		if _, err := buildProvider(ctx, kind, "k", "http://localhost/v1", "m"); errors.Is(err, ErrProviderUnsupported) {
			t.Errorf("kind %q has no adapter", kind)
		}
	}
	if _, err := buildProvider(ctx, "bogus", "k", "", "m"); !errors.Is(err, ErrProviderUnsupported) {
		t.Errorf("unknown kind should be unsupported, got %v", err)
	}
}

// registryProvider must yield a registry id for every vendor kind (custom = none).
func TestRegistryProviderPerKind(t *testing.T) {
	cases := map[models.AIProviderKind]string{
		models.AIProviderOpenAI:       "openai",
		models.AIProviderAnthropic:    "anthropic",
		models.AIProviderGoogle:       "google",
		models.AIProviderOpenAICompat: "",
	}
	for kind, want := range cases {
		if got := registryProvider(kind); got != want {
			t.Errorf("registryProvider(%q) = %q, want %q", kind, got, want)
		}
	}
}
