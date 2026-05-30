package kubernetes

import (
	"context"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

func pageRC(query url.Values) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, nil, map[string]string{}, query, nil)
}

func TestPageRowsSorts(t *testing.T) {
	mk := func() []Row {
		return []Row{
			{"name": "pod-b", "restarts": 5, "createdAt": "2026-01-02T00:00:00Z"},
			{"name": "pod-a", "restarts": 12, "createdAt": "2026-01-03T00:00:00Z"},
			{"name": "pod-c", "restarts": 1, "createdAt": "2026-01-01T00:00:00Z"},
		}
	}

	page, err := pageRows(pageRC(url.Values{"sort": {"name"}}), mk())
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0]["name"] != "pod-a" || page.Items[2]["name"] != "pod-c" {
		t.Errorf("name asc = %v", rowField(page.Items, "name"))
	}

	page, _ = pageRows(pageRC(url.Values{"sort": {"-restarts"}}), mk())
	if page.Items[0]["restarts"] != 12 || page.Items[2]["restarts"] != 1 {
		t.Errorf("restarts desc = %v", rowField(page.Items, "restarts"))
	}

	// Age ascending = youngest first = newest createdAt first (remapped + inverted).
	page, _ = pageRows(pageRC(url.Values{"sort": {"age"}}), mk())
	if page.Items[0]["createdAt"] != "2026-01-03T00:00:00Z" || page.Items[2]["createdAt"] != "2026-01-01T00:00:00Z" {
		t.Errorf("age asc should be youngest-first: %v", rowField(page.Items, "createdAt"))
	}
}

func rowField(rows []Row, key string) []any {
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r[key]
	}
	return out
}

func TestFilterRows(t *testing.T) {
	rows := []Row{{"name": "web-1"}, {"name": "api-2"}, {"name": "web-2"}}

	if got := filterRows(rows, "web"); len(got) != 2 {
		t.Fatalf("filter web = %d rows, want 2", len(got))
	}
	if got := filterRows(rows, "WEB"); len(got) != 2 {
		t.Fatalf("filter is not case-insensitive: %d rows", len(got))
	}
	if got := filterRows(rows, "  "); len(got) != 3 {
		t.Fatalf("blank filter must return all rows: %d", len(got))
	}
	if got := filterRows(rows, "none"); len(got) != 0 {
		t.Fatalf("non-matching filter must return no rows: %d", len(got))
	}
}
