package proxmox

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	resourceWatchInterval = 3 * time.Second
	taskWatchInterval     = 2 * time.Second
	storageWatchInterval  = 10 * time.Second
)

type watchedRow struct {
	ref         plugin.ResourceIdentity
	row         plugin.TableRow
	fingerprint string
}

func watchResource(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(rc.Param("kind"))
	if kind == "" {
		kind = strings.TrimSpace(rc.Query().Get("kind"))
	}
	if kind == "" {
		return fmt.Errorf("%w: kind is required", plugin.ErrInvalidInput)
	}
	interval := resourceWatchInterval
	switch kind {
	case "task":
		interval = taskWatchInterval
	case "storage":
		interval = storageWatchInterval
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	prev, err := watchedRows(rc, s, kind)
	if err != nil {
		return err
	}
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
			next, err := watchedRows(rc, s, kind)
			if err != nil {
				return err
			}
			events := diffWatchedRows(prev, next)
			prev = next
			for _, ev := range events {
				if err := enc.Encode(ev); err != nil {
					return nil
				}
			}
		}
	}
}

func watchedRows(rc *plugin.RequestContext, s *Session, kind string) (map[string]watchedRow, error) {
	node := strings.TrimSpace(rc.Query().Get("p.node"))
	if node == "" {
		node = strings.TrimSpace(rc.Query().Get("node"))
	}
	var rows []plugin.TableRow
	var err error
	switch kind {
	case "guest":
		rows, err = guestRows(rc.Ctx, s, "", node)
	case "qemu", "lxc":
		rows, err = guestRows(rc.Ctx, s, kind, node)
	case "node":
		rows, err = nodeRows(rc.Ctx, s)
	case "storage":
		rows, err = storageRows(rc.Ctx, s, node)
	case "task":
		var items []plugin.TableRow
		if node != "" {
			items, err = s.list(rc.Ctx, pvePath("nodes", node, "tasks"))
			for _, item := range items {
				if str(item["node"]) == "" {
					item["node"] = node
				}
			}
		} else {
			items, err = s.list(rc.Ctx, "/cluster/tasks")
		}
		if err == nil {
			rows = taskRows(items)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported watch kind %q", plugin.ErrInvalidInput, kind)
	}
	if err != nil {
		return nil, err
	}
	return indexWatchedRows(rows), nil
}

func watchObject(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	kind := strings.TrimSpace(rc.Param("kind"))
	node := strings.TrimSpace(rc.Param("node"))
	id := strings.TrimSpace(rc.Param("id"))
	if kind == "" || node == "" || id == "" {
		return fmt.Errorf("%w: kind, node, and id are required", plugin.ErrInvalidInput)
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(taskWatchInterval)
	defer ticker.Stop()

	row, _, err := objectWatchRow(rc, s, kind, node, id)
	if err != nil {
		return err
	}
	prev := fingerprint(row)
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
			var ref plugin.ResourceIdentity
			row, ref, err = objectWatchRow(rc, s, kind, node, id)
			if err != nil {
				return err
			}
			fp := fingerprint(row)
			if fp == prev {
				continue
			}
			prev = fp
			if err := enc.Encode(plugin.ResourceEvent{Type: "updated", Ref: ref, Resource: row}); err != nil {
				return nil
			}
		}
	}
}

func objectWatchRow(rc *plugin.RequestContext, s *Session, kind, node, id string) (plugin.TableRow, plugin.ResourceIdentity, error) {
	switch kind {
	case "qemu", "lxc":
		if !validNode(node) || !validVMID(id) {
			return nil, plugin.ResourceIdentity{}, fmt.Errorf("%w: invalid node or vmid", plugin.ErrInvalidInput)
		}
		result, err := guestOverview(kind)(plugin.NewRequestContext(rc.Ctx, rc.User, s, map[string]string{"node": node, "vmid": id}, nil, nil))
		if err != nil {
			return nil, plugin.ResourceIdentity{}, err
		}
		row := result.(plugin.TableRow)
		name := stringOr(str(row["name"]), kind+"/"+id)
		return row, plugin.ResourceIdentity{Kind: kind, Namespace: node, Name: name, UID: id}, nil
	case "node":
		if !validNode(node) {
			return nil, plugin.ResourceIdentity{}, fmt.Errorf("%w: invalid node", plugin.ErrInvalidInput)
		}
		result, err := nodeStatus(plugin.NewRequestContext(rc.Ctx, rc.User, s, map[string]string{"node": node}, nil, nil))
		if err != nil {
			return nil, plugin.ResourceIdentity{}, err
		}
		return result.(plugin.TableRow), plugin.ResourceIdentity{Kind: "node", Namespace: node, Name: node, UID: node}, nil
	case "task":
		if !validNode(node) || !validUPID(id) {
			return nil, plugin.ResourceIdentity{}, fmt.Errorf("%w: invalid node or task id", plugin.ErrInvalidInput)
		}
		result, err := taskStatus(plugin.NewRequestContext(rc.Ctx, rc.User, s, map[string]string{"node": node, "upid": id}, nil, nil))
		if err != nil {
			return nil, plugin.ResourceIdentity{}, err
		}
		row := result.(plugin.TableRow)
		return row, plugin.ResourceIdentity{Kind: "task", Namespace: node, Name: node, UID: id}, nil
	default:
		return nil, plugin.ResourceIdentity{}, fmt.Errorf("%w: unsupported watch kind %q", plugin.ErrInvalidInput, kind)
	}
}

func watchTimeline(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(taskWatchInterval)
	defer ticker.Stop()

	prev, err := timelineRows(rc, s)
	if err != nil {
		return err
	}
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
			next, err := timelineRows(rc, s)
			if err != nil {
				return err
			}
			events := diffWatchedRows(prev, next)
			prev = next
			for _, ev := range events {
				if err := enc.Encode(ev); err != nil {
					return nil
				}
			}
		}
	}
}

func timelineRows(rc *plugin.RequestContext, s *Session) (map[string]watchedRow, error) {
	items, err := s.list(rc.Ctx, "/cluster/tasks")
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(items))
	for _, task := range taskRows(items) {
		rows = append(rows, taskTimelineRow(task))
	}
	return indexWatchedRows(rows), nil
}

func watchTaskLog(rc *plugin.RequestContext, client plugin.ClientStream) error {
	s, err := sess(rc)
	if err != nil {
		return err
	}
	node, upid := rc.Param("node"), rc.Param("upid")
	if !validNode(node) || !validUPID(upid) {
		return fmt.Errorf("%w: invalid node or task id", plugin.ErrInvalidInput)
	}
	enc := json.NewEncoder(client)
	ticker := time.NewTicker(taskWatchInterval)
	defer ticker.Stop()

	seen, err := taskLogIndex(rc, s, node, upid)
	if err != nil {
		return err
	}
	for {
		select {
		case <-client.Context().Done():
			return nil
		case <-rc.Ctx.Done():
			return nil
		case <-ticker.C:
			next, err := taskLogIndex(rc, s, node, upid)
			if err != nil {
				return err
			}
			for key, row := range next {
				if _, ok := seen[key]; ok {
					continue
				}
				ref := plugin.ResourceIdentity{Kind: "task_log", Namespace: node, Name: upid, UID: key}
				if err := enc.Encode(plugin.ResourceEvent{Type: "added", Ref: ref, Resource: row}); err != nil {
					return nil
				}
			}
			seen = next
		}
	}
}

func taskLogIndex(rc *plugin.RequestContext, s *Session, node, upid string) (map[string]plugin.TableRow, error) {
	lines, err := s.list(rc.Ctx, pvePath("nodes", node, "tasks", upid, "log"))
	if err != nil {
		return nil, err
	}
	items := logRows(lines)
	rows := make(map[string]plugin.TableRow, len(items))
	for _, row := range items {
		key := fmt.Sprintf("%s:%d", upid, numInt(row["n"]))
		rows[key] = row
	}
	return rows, nil
}

func indexWatchedRows(rows []plugin.TableRow) map[string]watchedRow {
	out := make(map[string]watchedRow, len(rows))
	for _, row := range rows {
		ref, ok := row["ref"].(plugin.ResourceIdentity)
		if !ok || ref.UID == "" {
			continue
		}
		key := ref.Kind + ":" + ref.Namespace + ":" + ref.UID
		out[key] = watchedRow{ref: ref, row: row, fingerprint: fingerprint(row)}
	}
	return out
}

func diffWatchedRows(prev, next map[string]watchedRow) []plugin.ResourceEvent {
	events := make([]plugin.ResourceEvent, 0)
	for key, row := range next {
		old, ok := prev[key]
		if !ok {
			events = append(events, plugin.ResourceEvent{Type: "added", Ref: row.ref, Resource: row.row})
			continue
		}
		if old.fingerprint != row.fingerprint {
			events = append(events, plugin.ResourceEvent{Type: "updated", Ref: row.ref, Resource: row.row})
		}
	}
	for key, row := range prev {
		if _, ok := next[key]; !ok {
			events = append(events, plugin.ResourceEvent{Type: "deleted", Ref: row.ref})
		}
	}
	return events
}

func fingerprint(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}
