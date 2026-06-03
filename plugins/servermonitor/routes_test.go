package servermonitor

import (
	"context"
	"net/url"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func sampleRows() []hostmonitor.Row {
	return []hostmonitor.Row{
		{"_id": "p1", "name": "redis", "cpuPct": 10.0},
		{"_id": "p2", "name": "nginx", "cpuPct": 30.0},
		{"_id": "p3", "name": "postgres", "cpuPct": 20.0},
	}
}

func pageWith(t *testing.T, q url.Values, rows []hostmonitor.Row) plugin.Page[hostmonitor.Row] {
	t.Helper()
	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, nil, nil, q, nil)
	page, err := pageRows(rc, rows)
	if err != nil {
		t.Fatalf("pageRows: %v", err)
	}
	return page
}

func names(rows []hostmonitor.Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i], _ = r["name"].(string)
	}
	return out
}

func TestPageRowsSortDescending(t *testing.T) {
	page := pageWith(t, url.Values{"sort": {"-cpuPct"}}, sampleRows())
	got := names(page.Items)
	want := []string{"nginx", "postgres", "redis"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("sort -cpuPct = %v, want %v", got, want)
	}
}

func TestPageRowsSortAscendingString(t *testing.T) {
	page := pageWith(t, url.Values{"sort": {"name"}}, sampleRows())
	got := names(page.Items)
	want := []string{"nginx", "postgres", "redis"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort name = %v, want %v", got, want)
		}
	}
}

func TestPageRowsCursorPaginates(t *testing.T) {
	first := pageWith(t, url.Values{"sort": {"name"}, "limit": {"2"}}, sampleRows())
	if got := names(first.Items); len(got) != 2 || got[0] != "nginx" || got[1] != "postgres" {
		t.Fatalf("first page = %v", names(first.Items))
	}
	if first.NextCursor != "2" {
		t.Fatalf("next cursor = %q, want 2", first.NextCursor)
	}
	if first.Total == nil || *first.Total != 3 {
		t.Fatalf("total = %v, want 3", first.Total)
	}

	second := pageWith(t, url.Values{"sort": {"name"}, "limit": {"2"}, "cursor": {first.NextCursor}}, sampleRows())
	if got := names(second.Items); len(got) != 1 || got[0] != "redis" {
		t.Fatalf("second page = %v", names(second.Items))
	}
	if second.NextCursor != "" {
		t.Fatalf("expected no next cursor, got %q", second.NextCursor)
	}
}

func TestPageRowsFilterMatchesValuesNotMapWrapper(t *testing.T) {
	// "ngin" matches only nginx; a letter from the "map[" wrapper must not match.
	page := pageWith(t, url.Values{"filter": {"ngin"}}, sampleRows())
	if got := names(page.Items); len(got) != 1 || got[0] != "nginx" {
		t.Fatalf("filter ngin = %v, want [nginx]", got)
	}

	all := pageWith(t, url.Values{"filter": {"map["}}, sampleRows())
	if len(all.Items) != 0 {
		t.Fatalf("filter 'map[' matched %d rows, want 0", len(all.Items))
	}
}

func TestPageRowsRejectsBadCursor(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, nil, nil, url.Values{"cursor": {"abc"}}, nil)
	if _, err := pageRows(rc, sampleRows()); err == nil {
		t.Fatal("expected error for non-offset cursor")
	}
}
