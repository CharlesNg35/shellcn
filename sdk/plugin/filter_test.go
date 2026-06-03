package plugin

import "testing"

func TestFilterRows(t *testing.T) {
	rows := []map[string]any{
		{"name": "nginx", "partition": 10, "offset": int64(500), "ref": map[string]any{"uid": "10"}},
		{"name": "redis", "partition": 3, "offset": int64(99)},
		{"name": "worker-10", "partition": 7, "offset": int64(42)},
	}

	// Numbers are matched as visible cells: "10" hits partition 10 and "worker-10".
	got := FilterRows(rows, "10")
	if len(got) != 2 {
		t.Fatalf("filter 10 = %d rows, want 2 (partition 10, worker-10)", len(got))
	}

	// Text match is case-insensitive.
	if got := FilterRows(rows, "NGINX"); len(got) != 1 || got[0]["name"] != "nginx" {
		t.Fatalf("case-insensitive name filter = %#v", got)
	}

	// Identity fields are ignored: the ref carries uid "10" but isn't matched on
	// its own (row 1 still matches via partition 10, so use a uid-only term).
	if got := FilterRows(rows, "uid"); len(got) != 0 {
		t.Fatalf("reserved ref field must not match, got %#v", got)
	}

	// Empty term returns everything unchanged.
	if got := FilterRows(rows, "  "); len(got) != 3 {
		t.Fatalf("empty term = %d rows, want 3", len(got))
	}
}
