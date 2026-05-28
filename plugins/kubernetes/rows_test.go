package kubernetes

import "testing"

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
