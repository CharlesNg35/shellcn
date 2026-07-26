package sqldb

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestExactCountRequested(t *testing.T) {
	if ExactCountRequested(plugin.PageRequest{}) {
		t.Fatal("exact count must be opt-in")
	}
	if ExactCountRequested(plugin.PageRequest{Filter: map[string]string{CountFilterKey: "approx"}}) {
		t.Fatal("only the exact value opts in")
	}
	if !ExactCountRequested(plugin.PageRequest{Filter: map[string]string{CountFilterKey: " Exact "}}) {
		t.Fatal("filter.count=exact must opt in")
	}
}

func TestPageLimit(t *testing.T) {
	cases := []struct{ limit, rowLimit, want int }{
		{50, 1000, 50},
		{5000, 1000, 1000},
		{0, 1000, 1000},
		{-1, 1000, 1000},
		{0, 0, plugin.DefaultPageLimit},
		{25, 0, 25},
	}
	for _, c := range cases {
		if got := PageLimit(c.limit, c.rowLimit); got != c.want {
			t.Fatalf("PageLimit(%d, %d) = %d, want %d", c.limit, c.rowLimit, got, c.want)
		}
	}
}

func TestOverFetchAndTrim(t *testing.T) {
	if got := OverFetch(50); got != 51 {
		t.Fatalf("OverFetch(50) = %d", got)
	}
	if got := OverFetch(0); got != 0 {
		t.Fatalf("OverFetch(0) = %d", got)
	}
	full := []int{1, 2, 3, 4}
	items, more := TrimOverFetch(full, 3)
	if len(items) != 3 || !more {
		t.Fatalf("over-fetched page: items=%v more=%v", items, more)
	}
	items, more = TrimOverFetch([]int{1, 2}, 3)
	if len(items) != 2 || more {
		t.Fatalf("last page: items=%v more=%v", items, more)
	}
}

func TestOffsetPage(t *testing.T) {
	page := OffsetPage([]int{1, 2, 3}, 30, true, nil)
	if page.NextCursor != "33" {
		t.Fatalf("next cursor = %q", page.NextCursor)
	}
	if page.Total != nil {
		t.Fatalf("total must stay nil without an exact count: %v", *page.Total)
	}
	if page = OffsetPage([]int{1, 2, 3}, 30, false, nil); page.NextCursor != "" || page.Total != nil {
		t.Fatalf("later last page must not claim a total: %#v", page)
	}
	if page = OffsetPage([]int{1, 2, 3}, 0, false, nil); page.Total == nil || *page.Total != 3 {
		t.Fatalf("a collection that fit in one page knows its size: %#v", page)
	}
	total := 900
	page = OffsetPage([]int{1, 2, 3}, 30, true, &total)
	if page.Total == nil || *page.Total != 900 {
		t.Fatalf("requested exact count must survive: %#v", page)
	}
}
