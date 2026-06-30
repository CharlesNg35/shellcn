package plugin

import (
	"fmt"
	"strings"
)

// reservedFilterKeys are framework identity/navigation fields hidden from search.
var reservedFilterKeys = map[string]bool{
	"ref":       true,
	"_id":       true,
	"_key":      true,
	"_key_json": true,
	"_links":    true,
	"__rid":     true,
}

// FilterRows applies the table's case-insensitive free-text search.
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
