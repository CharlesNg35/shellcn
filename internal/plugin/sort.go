package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// SortRows orders rows in place by the first sort key: numeric cells compare
// numerically, all others case-insensitively as text. It lets a plugin that
// fetches a whole list honor the grid's column sort with no extra query. No keys
// → rows unchanged. The sort is stable, so equal cells keep their prior order.
func SortRows[R ~map[string]any](rows []R, keys []SortKey) []R {
	if len(keys) == 0 {
		return rows
	}
	k := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		c := compareCells(rows[i][k.Field], rows[j][k.Field])
		if k.Desc {
			return c > 0
		}
		return c < 0
	})
	return rows
}

// compareCells orders two cell values: numbers numerically, everything else by
// lower-cased text (a nil value reads as empty).
func compareCells(a, b any) int {
	if an, aok := cellNumber(a); aok {
		if bn, bok := cellNumber(b); bok {
			switch {
			case an < bn:
				return -1
			case an > bn:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(strings.ToLower(cellText(a)), strings.ToLower(cellText(b)))
}

func cellText(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func cellNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
