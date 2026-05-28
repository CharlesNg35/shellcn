package broker

import (
	"context"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

type row map[string]any

func pageRC(t *testing.T, filter string) *plugin.RequestContext {
	t.Helper()
	q := url.Values{}
	if filter != "" {
		q.Set("filter", filter)
	}
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, nil, nil, q, nil)
}

func TestPageRowsFiltersMapRows(t *testing.T) {
	rows := []row{{"name": "orders"}, {"name": "events"}, {"name": "order-dlq"}}

	page, err := PageRows(pageRC(t, "order"), rows)
	if err != nil {
		t.Fatalf("PageRows: %v", err)
	}
	if len(page.Items) != 2 || *page.Total != 2 {
		t.Fatalf("filter order = %d items (total %d), want 2", len(page.Items), *page.Total)
	}

	// Blank filter returns everything.
	all, _ := PageRows(pageRC(t, ""), rows)
	if len(all.Items) != 3 {
		t.Fatalf("no filter = %d items, want 3", len(all.Items))
	}
}

func TestPageRowsLeavesNonMapRowsUnfiltered(t *testing.T) {
	rows := [][]any{{"a"}, {"b"}}
	page, err := PageRows(pageRC(t, "zzz"), rows)
	if err != nil {
		t.Fatalf("PageRows: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("non-map rows must pass through unfiltered: got %d", len(page.Items))
	}
}
