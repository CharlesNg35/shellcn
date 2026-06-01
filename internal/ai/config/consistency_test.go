package aiconfig

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
)

// Every built-in vendor kind must have a default-model fallback for the picker;
// openai_compatible is a custom endpoint and intentionally has none.
func TestDefaultModelsCoverVendorKinds(t *testing.T) {
	for kind := range builtinKinds {
		_, ok := defaultModels[kind]
		if kind == models.AIProviderOpenAICompat {
			if ok {
				t.Errorf("openai_compatible should not define default models")
			}
			continue
		}
		if !ok || len(defaultModels[kind]) == 0 {
			t.Errorf("built-in kind %q has no default models", kind)
		}
	}
}
