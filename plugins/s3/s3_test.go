package s3

import (
	"testing"

	"github.com/charlesng35/shellcn/internal/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}
