package servermonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
)

const permRead = "server_monitor.read"

func Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "server_monitor.overview", Method: plugin.MethodGet, Path: "/overview", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.overview", Handle: Overview},
		{ID: "server_monitor.processes", Method: plugin.MethodGet, Path: "/processes", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.processes", Handle: Processes},
		{ID: "server_monitor.services", Method: plugin.MethodGet, Path: "/services", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.services", Handle: Services},
		{ID: "server_monitor.disks", Method: plugin.MethodGet, Path: "/disks", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disks", Handle: Disks},
		{ID: "server_monitor.disk_io", Method: plugin.MethodGet, Path: "/disk-io", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disk_io", Handle: DiskIO},
		{ID: "server_monitor.network", Method: plugin.MethodGet, Path: "/network", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.network", Handle: Networks},
		{ID: "server_monitor.connections", Method: plugin.MethodGet, Path: "/connections", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.connections", Handle: Connections},
		{ID: "server_monitor.users", Method: plugin.MethodGet, Path: "/users", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.users", Handle: Users},
		{ID: "server_monitor.sensors", Method: plugin.MethodGet, Path: "/sensors", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.sensors", Handle: Sensors},
		{ID: "server_monitor.cpu", Method: plugin.MethodGet, Path: "/cpu", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.cpu", Handle: CPUInfo},
		{ID: "server_monitor.metrics", Method: plugin.MethodWS, Path: "/metrics", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.metrics", Stream: Metrics},
	}
}

func Overview(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	return s.backend.Overview(rc.Ctx)
}

func Processes(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Processes(ctx) })
}

func Services(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Services(ctx) })
}

func Disks(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Disks(ctx) })
}

func DiskIO(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.DiskIO(ctx) })
}

func Networks(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Networks(ctx) })
}

func Connections(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Connections(ctx) })
}

func Users(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Users(ctx) })
}

func Sensors(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Sensors(ctx) })
}

func CPUInfo(rc *plugin.RequestContext) (any, error) {
	return rowsPage(rc, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.CPUInfo(ctx) })
}

func rowsPage(rc *plugin.RequestContext, fetch func(context.Context, hostmonitor.Backend) ([]hostmonitor.Row, error)) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	rows, err := fetch(rc.Ctx, s.backend)
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func Metrics(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	var prev map[string]any
	var prevAt time.Time
	for {
		frame, err := s.backend.Metrics(rc.Ctx)
		if err != nil {
			return err
		}
		now := time.Now()
		if dt := now.Sub(prevAt).Seconds(); prev != nil && dt > 0 {
			addRate(frame, prev, "netBytesRecv", "netRecvRate", dt)
			addRate(frame, prev, "netBytesSent", "netSentRate", dt)
			addRate(frame, prev, "diskReadBytes", "diskReadRate", dt)
			addRate(frame, prev, "diskWriteBytes", "diskWriteRate", dt)
		}
		prev, prevAt = frame, now
		if err := enc.Encode(frame); err != nil {
			return nil
		}
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// addRate derives a per-second rate from a monotonic counter and the previous
// frame, so cumulative byte totals become a meaningful live series. A counter
// reset (reboot/wrap) yields a negative delta and is skipped.
func addRate(frame, prev map[string]any, cumKey, rateKey string, dt float64) {
	cur, ok1 := number(frame[cumKey])
	old, ok2 := number(prev[cumKey])
	if ok1 && ok2 && cur >= old {
		frame[rateKey] = (cur - old) / dt
	}
}

func pageRows(rc *plugin.RequestContext, rows []hostmonitor.Row) (plugin.Page[hostmonitor.Row], error) {
	req, err := rc.Page()
	if err != nil {
		return plugin.Page[hostmonitor.Row]{}, err
	}
	rows = filterRows(rows, req.Filter["q"])
	sortRows(rows, req.Sort)
	start := 0
	if req.Cursor != "" {
		start, err = strconv.Atoi(req.Cursor)
		if err != nil || start < 0 {
			return plugin.Page[hostmonitor.Row]{}, fmt.Errorf("%w: cursor must be an offset", plugin.ErrInvalidInput)
		}
	}
	total := len(rows)
	if start > total {
		start = total
	}
	end := start + req.Limit
	if end > total {
		end = total
	}
	next := ""
	if end < total {
		next = strconv.Itoa(end)
	}
	return plugin.Page[hostmonitor.Row]{Items: rows[start:end], NextCursor: next, Total: &total}, nil
}

func filterRows(rows []hostmonitor.Row, q string) []hostmonitor.Row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]hostmonitor.Row, 0, len(rows))
	for _, r := range rows {
		if rowMatches(r, q) {
			out = append(out, r)
		}
	}
	return out
}

// rowMatches tests the query against a row's values, not its map representation,
// so the "map[" / key-name noise of fmt.Sprint(row) can't match everything.
func rowMatches(r hostmonitor.Row, q string) bool {
	for k, v := range r {
		if k == "_id" {
			continue
		}
		if strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
			return true
		}
	}
	return false
}

func sortRows(rows []hostmonitor.Row, keys []plugin.SortKey) {
	if len(keys) == 0 {
		return
	}
	key := keys[0]
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := compare(rows[i][key.Field], rows[j][key.Field])
		if key.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compare(a, b any) int {
	if an, ok := number(a); ok {
		if bn, ok := number(b); ok {
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
	as, bs := strings.ToLower(fmt.Sprint(a)), strings.ToLower(fmt.Sprint(b))
	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	default:
		return 0
	}
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
