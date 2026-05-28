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
		{ID: "server_monitor.processes.watch", Method: plugin.MethodWS, Path: "/processes/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.processes.watch", Stream: ProcessWatch},
		{ID: "server_monitor.services", Method: plugin.MethodGet, Path: "/services", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.services", Handle: Services},
		{ID: "server_monitor.services.watch", Method: plugin.MethodWS, Path: "/services/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.services.watch", Stream: ServiceWatch},
		{ID: "server_monitor.disks", Method: plugin.MethodGet, Path: "/disks", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disks", Handle: Disks},
		{ID: "server_monitor.disks.watch", Method: plugin.MethodWS, Path: "/disks/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disks.watch", Stream: DiskWatch},
		{ID: "server_monitor.disk_io", Method: plugin.MethodGet, Path: "/disk-io", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disk_io", Handle: DiskIO},
		{ID: "server_monitor.disk_io.watch", Method: plugin.MethodWS, Path: "/disk-io/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.disk_io.watch", Stream: DiskIOWatch},
		{ID: "server_monitor.network", Method: plugin.MethodGet, Path: "/network", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.network", Handle: Networks},
		{ID: "server_monitor.network.watch", Method: plugin.MethodWS, Path: "/network/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.network.watch", Stream: NetworkWatch},
		{ID: "server_monitor.connections", Method: plugin.MethodGet, Path: "/connections", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.connections", Handle: Connections},
		{ID: "server_monitor.connections.watch", Method: plugin.MethodWS, Path: "/connections/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.connections.watch", Stream: ConnectionWatch},
		{ID: "server_monitor.users", Method: plugin.MethodGet, Path: "/users", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.users", Handle: Users},
		{ID: "server_monitor.users.watch", Method: plugin.MethodWS, Path: "/users/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.users.watch", Stream: UserWatch},
		{ID: "server_monitor.sensors", Method: plugin.MethodGet, Path: "/sensors", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.sensors", Handle: Sensors},
		{ID: "server_monitor.sensors.watch", Method: plugin.MethodWS, Path: "/sensors/watch", Permission: permRead, Risk: plugin.RiskSafe, AuditEvent: "server_monitor.sensors.watch", Stream: SensorWatch},
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
	for {
		frame, err := s.backend.Metrics(rc.Ctx)
		if err != nil {
			return err
		}
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

func ProcessWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Processes(ctx) })
}

func ServiceWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Services(ctx) })
}

func DiskWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Disks(ctx) })
}

func DiskIOWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.DiskIO(ctx) })
}

func NetworkWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Networks(ctx) })
}

func ConnectionWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Connections(ctx) })
}

func UserWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Users(ctx) })
}

func SensorWatch(rc *plugin.RequestContext, client plugin.ClientStream) error {
	return watchRows(rc, client, func(ctx context.Context, b hostmonitor.Backend) ([]hostmonitor.Row, error) { return b.Sensors(ctx) })
}

func watchRows(rc *plugin.RequestContext, client plugin.ClientStream, fetch func(context.Context, hostmonitor.Backend) ([]hostmonitor.Row, error)) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	seen := map[string]string{}
	for {
		rows, err := fetch(rc.Ctx, s.backend)
		if err != nil {
			return err
		}
		next := make(map[string]string, len(rows))
		for _, row := range rows {
			ref, ok := rowRef(row)
			if !ok {
				continue
			}
			hash := rowHash(row)
			next[ref.UID] = hash
			eventType := "updated"
			if _, ok := seen[ref.UID]; !ok {
				eventType = "added"
			} else if seen[ref.UID] == hash {
				continue
			}
			if err := enc.Encode(plugin.ResourceEvent{Type: eventType, Ref: ref, Resource: row}); err != nil {
				return nil
			}
		}
		for uid := range seen {
			if _, ok := next[uid]; !ok {
				if err := enc.Encode(plugin.ResourceEvent{Type: "deleted", Ref: plugin.ResourceRef{UID: uid}}); err != nil {
					return nil
				}
			}
		}
		seen = next
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
		}
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

func rowRef(row hostmonitor.Row) (plugin.ResourceRef, bool) {
	raw, ok := row["ref"]
	if !ok {
		return plugin.ResourceRef{}, false
	}
	switch ref := raw.(type) {
	case plugin.ResourceRef:
		return ref, ref.UID != ""
	case map[string]string:
		return plugin.ResourceRef{Kind: ref["kind"], Name: ref["name"], UID: ref["uid"]}, ref["uid"] != ""
	case map[string]any:
		uid := fmt.Sprint(ref["uid"])
		if uid == "" || uid == "<nil>" {
			return plugin.ResourceRef{}, false
		}
		return plugin.ResourceRef{Kind: fmt.Sprint(ref["kind"]), Name: fmt.Sprint(ref["name"]), UID: uid}, true
	default:
		return plugin.ResourceRef{}, false
	}
}

func rowHash(row hostmonitor.Row) string {
	b, _ := json.Marshal(row)
	return string(b)
}

func filterRows(rows []hostmonitor.Row, q string) []hostmonitor.Row {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]hostmonitor.Row, 0, len(rows))
	for _, r := range rows {
		if strings.Contains(strings.ToLower(fmt.Sprint(r)), q) {
			out = append(out, r)
		}
	}
	return out
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
