package kubernetes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// Row is one table/tree record: a flat field map the generic renderer displays.
type Row map[string]any

func ptr[T any](v T) *T { return &v }

// pageRows applies offset-cursor pagination over an in-memory row slice, the
// same contract the generic table panel expects (items + nextCursor + total).
// Kubernetes lists are fetched whole (and kept live by watch); this slices them
// for the grid without a second server round-trip.
func pageRows(rc *plugin.RequestContext, rows []Row) (plugin.Page[Row], error) {
	page, err := rc.Page()
	if err != nil {
		return plugin.Page[Row]{}, err
	}
	rows = filterRows(rows, page.Search())
	start := 0
	if page.Cursor != "" {
		start, err = strconv.Atoi(page.Cursor)
		if err != nil || start < 0 {
			return plugin.Page[Row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
		}
	}
	total := len(rows)
	if start > total {
		start = total
	}
	end := start + page.Limit
	if end > total {
		end = total
	}
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return plugin.Page[Row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

// filterRows keeps rows whose string fields contain the query (case-insensitive),
// backing the table's filter box over the in-memory list the grid paginates.
func filterRows(rows []Row, q string) []Row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		for _, v := range r {
			if s, ok := v.(string); ok && strings.Contains(strings.ToLower(s), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
