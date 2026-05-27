package kubernetes

import (
	"fmt"
	"strconv"

	"github.com/charlesng/shellcn/internal/plugin"
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
