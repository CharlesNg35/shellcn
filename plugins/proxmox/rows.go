package proxmox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	pmox "github.com/luthermonson/go-proxmox"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type row = map[string]any

func sess(rc *plugin.RequestContext) (*Session, error) {
	return Unwrap(rc.Session)
}

// list GETs a JSON array endpoint into rows.
func (s *Session) list(ctx context.Context, path string) ([]row, error) {
	var out []row
	if err := s.client.Get(ctx, path, &out); err != nil {
		return nil, mapErr(err)
	}
	return out, nil
}

// object GETs a single JSON object endpoint.
func (s *Session) object(ctx context.Context, path string) (row, error) {
	var out row
	if err := s.client.Get(ctx, path, &out); err != nil {
		return nil, mapErr(err)
	}
	return out, nil
}

func (s *Session) post(ctx context.Context, path string, body any) error {
	return mapErr(s.client.Post(ctx, path, body, nil))
}

func (s *Session) del(ctx context.Context, path string) error {
	return mapErr(s.client.Delete(ctx, path, nil))
}

// mapErr translates go-proxmox sentinels into the core's sentinel errors so the
// server boundary renders the right status code.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, pmox.ErrNotFound):
		return fmt.Errorf("%w: %v", plugin.ErrNotFound, err)
	case errors.Is(err, pmox.ErrNotAuthorized):
		return fmt.Errorf("%w: %v", plugin.ErrForbidden, err)
	case errors.Is(err, pmox.ErrTimeout):
		return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	default:
		return err
	}
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}

func numFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}

func numInt(v any) int64 { return int64(numFloat(v)) }

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// pageRows applies filter/sort/cursor pagination to in-memory rows, mirroring the
// Docker plugin's list contract so the generic table panel behaves identically.
func pageRows(rc *plugin.RequestContext, rows []row) (plugin.Page[row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[row]{}, err
	}
	rows = filterRows(rows, req.Search())
	sortRows(rows, req.Sort)
	total := len(rows)
	start := 0
	if req.Cursor != "" {
		start, err = strconv.Atoi(req.Cursor)
		if err != nil || start < 0 {
			return plugin.Page[row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
		}
	}
	if start > len(rows) {
		start = len(rows)
	}
	end := min(start+req.Limit, len(rows))
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return plugin.Page[row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []row, q string) []row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		if strings.Contains(strings.ToLower(fmt.Sprint(r)), q) {
			out = append(out, r)
		}
	}
	return out
}

func sortRows(rows []row, keys []plugin.SortKey) {
	if len(keys) == 0 {
		keys = []plugin.SortKey{{Field: "name"}}
	}
	key := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i][key.Field], rows[j][key.Field]
		if af, ok := a.(float64); ok {
			if bf, ok := b.(float64); ok {
				if key.Desc {
					return af > bf
				}
				return af < bf
			}
		}
		as, bs := fmt.Sprint(a), fmt.Sprint(b)
		if key.Desc {
			return as > bs
		}
		return as < bs
	})
}

func ignoreEOF(err error) error {
	if err == nil || err == io.EOF || err == io.ErrUnexpectedEOF {
		return nil
	}
	return err
}

func ptr[T any](v T) *T { return &v }
