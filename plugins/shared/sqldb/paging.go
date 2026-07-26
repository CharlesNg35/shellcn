package sqldb

import (
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// CountFilterKey is the page filter a caller sets (filter.count=exact) to ask a
// table listing for a precise row count. Counting a SQL table is a full scan on
// every page and every refresh, so it is opt-in.
const CountFilterKey = "count"

// CountExact is the CountFilterKey value that opts into the exact count.
const CountExact = "exact"

// ExactCountRequested reports whether the request opted into the exact row count.
func ExactCountRequested(p plugin.PageRequest) bool {
	return strings.EqualFold(strings.TrimSpace(p.Filter[CountFilterKey]), CountExact)
}

// PageLimit clamps a requested page size to the plugin's row limit, falling back
// to it when the request omits one.
func PageLimit(limit, rowLimit int) int {
	if rowLimit > 0 && (limit <= 0 || limit > rowLimit) {
		return rowLimit
	}
	if limit <= 0 {
		return plugin.DefaultPageLimit
	}
	return limit
}

// OverFetch is the row count to ask the driver for when serving a page of size
// limit: one extra row, whose presence means another page follows — cheaper than
// counting the table just to know whether a next cursor exists.
func OverFetch(limit int) int {
	if limit <= 0 {
		return 0
	}
	return limit + 1
}

// TrimOverFetch cuts an over-fetched result back to one page and reports whether
// more rows follow.
func TrimOverFetch[T any](rows []T, limit int) ([]T, bool) {
	if limit > 0 && len(rows) > limit {
		return rows[:limit], true
	}
	return rows, false
}

// OffsetPage assembles a bounded offset-cursor page. total stays nil — the grid
// then pages by cursor — unless the caller opted into an exact count, or the
// whole collection fit in the first page, where its size is exact and free.
func OffsetPage[T any](items []T, offset int, more bool, total *int) plugin.Page[T] {
	next := ""
	if more {
		next = strconv.Itoa(offset + len(items))
	}
	if total == nil && offset == 0 && !more {
		n := len(items)
		total = &n
	}
	return plugin.Page[T]{Items: items, NextCursor: next, Total: total}
}
