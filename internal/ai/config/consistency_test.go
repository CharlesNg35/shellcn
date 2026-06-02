package aiconfig

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
)

// Every built-in vendor kind must have a model fallback for the picker;
// openai_compatible is a custom endpoint and intentionally has none.
func TestVendorModelCatalogCoversVendorKinds(t *testing.T) {
	for kind := range builtinKinds {
		_, ok := vendorModelCatalog[kind]
		if kind == models.AIProviderOpenAICompat {
			if ok {
				t.Errorf("openai_compatible should not define fallback models")
			}
			continue
		}
		if !ok || len(vendorModelCatalog[kind]) == 0 {
			t.Errorf("built-in kind %q has no fallback models", kind)
		}
	}
}
