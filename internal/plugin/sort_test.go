package plugin

import "testing"

func TestSortRows(t *testing.T) {
	rows := func() []map[string]any {
		return []map[string]any{
			{"name": "beta", "n": 2},
			{"name": "Alpha", "n": 10},
			{"name": "gamma", "n": 1},
		}
	}

	asc := SortRows(rows(), []SortKey{{Field: "name"}})
	if asc[0]["name"] != "Alpha" || asc[1]["name"] != "beta" || asc[2]["name"] != "gamma" {
		t.Errorf("text asc (case-insensitive) = %v", names(asc))
	}

	desc := SortRows(rows(), []SortKey{{Field: "name", Desc: true}})
	if desc[0]["name"] != "gamma" || desc[2]["name"] != "Alpha" {
		t.Errorf("text desc = %v", names(desc))
	}

	// Numbers compare numerically, not lexically (10 > 2, not "10" < "2").
	num := SortRows(rows(), []SortKey{{Field: "n"}})
	if num[0]["n"] != 1 || num[1]["n"] != 2 || num[2]["n"] != 10 {
		t.Errorf("numeric asc = %v", num)
	}

	if got := SortRows(rows(), nil); got[0]["name"] != "beta" {
		t.Errorf("no keys should leave order unchanged, got %v", names(got))
	}
}

func names(rows []map[string]any) []any {
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r["name"]
	}
	return out
}
