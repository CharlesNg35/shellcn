package plugin

import (
	"fmt"
	"strings"
)

// reservedFilterKeys are framework identity/navigation fields the table search
// must ignore — they're opaque (a ResourceRef, a key map, an internal id), not
// user-visible cell content.
var reservedFilterKeys = map[string]bool{
	"ref":    true,
	"_id":    true,
	"_key":   true,
	"_links": true,
	"__rid":  true,
}

// FilterRows keeps the rows whose any visible cell contains the term
// (case-insensitive substring) — the grid's free-text search. It matches every
// field as the user sees it (numbers, dates, text), skipping only the reserved
// identity fields, so behavior is identical across every plugin. Empty term
// returns the rows unchanged.
func FilterRows[R ~map[string]any](rows []R, term string) []R {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return rows
	}
	out := make([]R, 0, len(rows))
	for _, r := range rows {
		for k, v := range r {
			if reservedFilterKeys[k] {
				continue
			}
			if strings.Contains(strings.ToLower(fmt.Sprint(v)), term) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
