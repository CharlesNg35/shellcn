package service

import (
	"testing"

	"github.com/charlesng/shellcn/internal/models"
)

func TestConnectionSlugIsUniqueAndDNSSafe(t *testing.T) {
	a := connectionSlug(models.Connection{Name: "Prod Cluster (Café)", ID: "11111111-2222-3333-4444-555555555555"})
	if a != "prod-cluster-cafe-11111111" {
		t.Fatalf("slug = %q", a)
	}
	// Same name, different connection id → distinct slugs (no clobber).
	b := connectionSlug(models.Connection{Name: "Prod Cluster (Café)", ID: "99999999-2222-3333-4444-555555555555"})
	if a == b {
		t.Fatalf("same-name connections must get distinct slugs: %q == %q", a, b)
	}
	// A name that folds to nothing still yields a valid DNS-1123 label.
	if got := connectionSlug(models.Connection{Name: "***", ID: "abcdef01-0000-0000-0000-000000000000"}); got != "shellcn-agent-abcdef01" {
		t.Fatalf("degenerate slug = %q", got)
	}
}
