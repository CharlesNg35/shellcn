package kubernetes

import (
	"fmt"
	"strconv"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Row is one table/tree record: a flat field map the generic renderer displays.
type Row map[string]any

func ptr[T any](v T) *T { return &v }

// maxMaterializedRows caps how many rows a handler may hold in memory for one
// page. It matches the SDK's page ceiling, so anything larger is a fetch that
// was never bounded in the first place.
const maxMaterializedRows = plugin.MaxPageLimit

// pageRows applies offset-cursor pagination over rows a handler already holds,
// the same contract the generic table panel expects (items + nextCursor + total).
// Callers must fetch a bounded window; rows past the cap are dropped, so this can
// never turn into a whole-collection read.
func pageRows(rc *plugin.RequestContext, rows []Row) (plugin.Page[Row], error) {
	return pageSlice(rc, rows, true)
}

// pageSlice is pageRows with an explicit statement of whether rows is the whole
// collection. When it isn't, Total is omitted so the grid pages forward through a
// window instead of rendering a partial count as the collection size.
func pageSlice(rc *plugin.RequestContext, rows []Row, complete bool) (plugin.Page[Row], error) {
	page, err := rc.Page()
	if err != nil {
		return plugin.Page[Row]{}, err
	}
	rows = filterRows(rows, page.Search())
	rows = plugin.SortRows(rows, sortKeys(page.Sort))
	if len(rows) > maxMaterializedRows {
		rows, complete = rows[:maxMaterializedRows], false
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
	out := plugin.Page[Row]{Items: rows[start:end], NextCursor: next}
	if complete {
		out.Total = &total
	}
	return out, nil
}

// pageWindow returns a page the server already bounded, with next being the
// driver's own cursor. Filter and sort therefore apply to that window, not the
// collection, and Total is reported only when the window held everything.
func pageWindow(rc *plugin.RequestContext, rows []Row, next string) (plugin.Page[Row], error) {
	page, err := rc.Page()
	if err != nil {
		return plugin.Page[Row]{}, err
	}
	rows = filterRows(rows, page.Search())
	rows = plugin.SortRows(rows, sortKeys(page.Sort))
	if rows == nil {
		rows = []Row{}
	}
	out := plugin.Page[Row]{Items: rows, NextCursor: next}
	if next == "" && page.Cursor == "" {
		out.Total = ptr(len(rows))
	}
	return out, nil
}

// filterRows keeps rows whose string fields contain the query (case-insensitive),
// backing the table's filter box over the in-memory list the grid paginates.
func filterRows(rows []Row, q string) []Row {
	return plugin.FilterRows(rows, q)
}

// sortKeys remaps the human "age" column to its underlying creation timestamp
// (inverted, so ascending age is youngest-first) — its displayed value is a
// relative string that wouldn't compare correctly. Other columns sort as-is.
func sortKeys(keys []plugin.SortKey) []plugin.SortKey {
	out := make([]plugin.SortKey, len(keys))
	for i, k := range keys {
		if k.Field == "age" {
			k.Field, k.Desc = "createdAt", !k.Desc
		}
		out[i] = k
	}
	return out
}
