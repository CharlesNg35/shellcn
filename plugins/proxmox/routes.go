package proxmox

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type actionResult struct {
	OK bool `json:"ok"`
}

func Routes() []plugin.Route {
	routes := []plugin.Route{
		// Tree.
		{ID: "proxmox.tree.nodes", Method: plugin.MethodGet, Path: "/tree/nodes", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.tree.nodes", Handle: treeNodes},
		{ID: "proxmox.tree.storage", Method: plugin.MethodGet, Path: "/tree/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.tree.storage", Handle: treeStorage},

		// Lists.
		{ID: "proxmox.guest.list", Method: plugin.MethodGet, Path: "/guests", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.guest.list", Handle: listGuests("")},
		{ID: "proxmox.qemu.list", Method: plugin.MethodGet, Path: "/qemu", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.list", Handle: listGuests("qemu")},
		{ID: "proxmox.lxc.list", Method: plugin.MethodGet, Path: "/lxc", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.list", Handle: listGuests("lxc")},
		{ID: "proxmox.node.list", Method: plugin.MethodGet, Path: "/nodes", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.list", Handle: listNodes},
		{ID: "proxmox.node.options", Method: plugin.MethodGet, Path: "/nodes/options", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.options", Handle: nodeOptions},
		{ID: "proxmox.node.backup_storage.options", Method: plugin.MethodGet, Path: "/nodes/{node}/backup-storage/options", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.backup_storage.options", Handle: backupStorageOptions},
		{ID: "proxmox.node.guest_storage.options", Method: plugin.MethodGet, Path: "/nodes/{node}/guest-storage/options", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.guest_storage.options", Handle: guestStorageOptions},
		{ID: "proxmox.storage.list", Method: plugin.MethodGet, Path: "/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.storage.list", Handle: listStorage},
		{ID: "proxmox.node.storage", Method: plugin.MethodGet, Path: "/nodes/{node}/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.storage", Handle: listNodeStorage},
		{ID: "proxmox.node.tasks", Method: plugin.MethodGet, Path: "/nodes/{node}/tasks", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.tasks", Handle: listTasks},
		{ID: "proxmox.task.list", Method: plugin.MethodGet, Path: "/tasks", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.task.list", Handle: listClusterTasks},
		{ID: "proxmox.storage.content", Method: plugin.MethodGet, Path: "/nodes/{node}/storage/{storage}/content", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.storage.content", Handle: listStorageContent},
		{ID: "proxmox.qemu.snapshots", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/snapshot", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.snapshots", Handle: listSnapshots("qemu")},
		{ID: "proxmox.lxc.snapshots", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/snapshot", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.snapshots", Handle: listSnapshots("lxc")},
		{ID: "proxmox.qemu.backups", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/backups", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.backups", Handle: listBackups},
		{ID: "proxmox.lxc.backups", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/backups", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.backups", Handle: listBackups},

		// Documents.
		{ID: "proxmox.qemu.config", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/config", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.config", Handle: guestConfig("qemu")},
		{ID: "proxmox.lxc.config", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/config", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.config", Handle: guestConfig("lxc")},
		{ID: "proxmox.qemu.overview", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/overview", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.overview", Handle: guestOverview("qemu")},
		{ID: "proxmox.lxc.overview", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/overview", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.overview", Handle: guestOverview("lxc")},
		{ID: "proxmox.node.status", Method: plugin.MethodGet, Path: "/nodes/{node}/status", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.status", Handle: nodeStatus},

		// Metrics streams.
		{ID: "proxmox.qemu.metrics", Method: plugin.MethodWS, Path: "/nodes/{node}/qemu/{vmid}/metrics", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.metrics", Stream: guestMetrics("qemu")},
		{ID: "proxmox.lxc.metrics", Method: plugin.MethodWS, Path: "/nodes/{node}/lxc/{vmid}/metrics", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.metrics", Stream: guestMetrics("lxc")},
		{ID: "proxmox.node.metrics", Method: plugin.MethodWS, Path: "/nodes/{node}/metrics", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.metrics", Stream: nodeMetrics},

		// Consoles.
		{ID: "proxmox.qemu.console", Method: plugin.MethodWS, Path: "/nodes/{node}/qemu/{vmid}/console", Permission: "proxmox.qemu.console", Risk: plugin.RiskPrivileged, AuditEvent: "proxmox.qemu.console", Stream: vmConsole},
		{ID: "proxmox.lxc.console", Method: plugin.MethodWS, Path: "/nodes/{node}/lxc/{vmid}/console", Permission: "proxmox.lxc.console", Risk: plugin.RiskPrivileged, AuditEvent: "proxmox.lxc.console", Stream: terminalConsole("lxc")},
		{ID: "proxmox.node.shell", Method: plugin.MethodWS, Path: "/nodes/{node}/shell", Permission: "proxmox.node.shell", Risk: plugin.RiskPrivileged, AuditEvent: "proxmox.node.shell", Stream: terminalConsole("node")},

		// Backups + migrate.
		{ID: "proxmox.qemu.backup", Method: plugin.MethodPost, Path: "/nodes/{node}/qemu/{vmid}/backup", Permission: "proxmox.backup.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.qemu.backup", Input: backupSchema(), Handle: backupCreate},
		{ID: "proxmox.lxc.backup", Method: plugin.MethodPost, Path: "/nodes/{node}/lxc/{vmid}/backup", Permission: "proxmox.backup.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.lxc.backup", Input: backupSchema(), Handle: backupCreate},
		{ID: "proxmox.backup.delete", Method: plugin.MethodDelete, Path: "/nodes/{node}/storage/{storage}/content/{volume}", Permission: "proxmox.backup.delete", Risk: plugin.RiskDestructive, AuditEvent: "proxmox.backup.delete", Handle: backupDelete},
		{ID: "proxmox.qemu.migrate", Method: plugin.MethodPost, Path: "/nodes/{node}/qemu/{vmid}/migrate", Permission: "proxmox.qemu.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.qemu.migrate", Input: migrateSchema(), Handle: guestMigrate("qemu")},
		{ID: "proxmox.lxc.migrate", Method: plugin.MethodPost, Path: "/nodes/{node}/lxc/{vmid}/migrate", Permission: "proxmox.lxc.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.lxc.migrate", Input: migrateSchema(), Handle: guestMigrate("lxc")},

		// Clone / restore / destroy / resize.
		{ID: "proxmox.qemu.clone", Method: plugin.MethodPost, Path: "/nodes/{node}/qemu/{vmid}/clone", Permission: "proxmox.qemu.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.qemu.clone", Input: cloneSchema("qemu"), Handle: guestClone("qemu")},
		{ID: "proxmox.lxc.clone", Method: plugin.MethodPost, Path: "/nodes/{node}/lxc/{vmid}/clone", Permission: "proxmox.lxc.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.lxc.clone", Input: cloneSchema("lxc"), Handle: guestClone("lxc")},
		{ID: "proxmox.qemu.restore", Method: plugin.MethodPost, Path: "/nodes/{node}/qemu", Permission: "proxmox.qemu.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.qemu.restore", Input: restoreSchema("qemu"), Handle: guestRestore("qemu")},
		{ID: "proxmox.lxc.restore", Method: plugin.MethodPost, Path: "/nodes/{node}/lxc", Permission: "proxmox.lxc.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.lxc.restore", Input: restoreSchema("lxc"), Handle: guestRestore("lxc")},
		{ID: "proxmox.qemu.destroy", Method: plugin.MethodDelete, Path: "/nodes/{node}/qemu/{vmid}", Permission: "proxmox.qemu.write", Risk: plugin.RiskDestructive, AuditEvent: "proxmox.qemu.destroy", Handle: guestDestroy("qemu")},
		{ID: "proxmox.lxc.destroy", Method: plugin.MethodDelete, Path: "/nodes/{node}/lxc/{vmid}", Permission: "proxmox.lxc.write", Risk: plugin.RiskDestructive, AuditEvent: "proxmox.lxc.destroy", Handle: guestDestroy("lxc")},
		{ID: "proxmox.qemu.resize", Method: plugin.MethodPut, Path: "/nodes/{node}/qemu/{vmid}/resize", Permission: "proxmox.qemu.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.qemu.resize", Input: resizeSchema(), Handle: qemuResize},

		// Node power.
		{ID: "proxmox.node.power", Method: plugin.MethodPost, Path: "/nodes/{node}/status", Permission: "proxmox.node.write", Risk: plugin.RiskDestructive, AuditEvent: "proxmox.node.power", Input: powerSchema(), Handle: nodePower},

		// Task control.
		{ID: "proxmox.task.stop", Method: plugin.MethodDelete, Path: "/nodes/{node}/tasks/{upid}", Permission: "proxmox.task.write", Risk: plugin.RiskWrite, AuditEvent: "proxmox.task.stop", Handle: taskStop},
		{ID: "proxmox.task.status", Method: plugin.MethodGet, Path: "/nodes/{node}/tasks/{upid}/status", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.task.status", Handle: taskStatus},
		{ID: "proxmox.task.log", Method: plugin.MethodGet, Path: "/nodes/{node}/tasks/{upid}/log", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.task.log", Handle: taskLog},
	}

	// Lifecycle (per guest kind).
	for _, kind := range []string{"qemu", "lxc"} {
		routes = append(routes,
			statusRoute(kind, "start", plugin.RiskWrite, false),
			statusRoute(kind, "shutdown", plugin.RiskWrite, true),
			statusRoute(kind, "stop", plugin.RiskDestructive, true),
			statusRoute(kind, "reboot", plugin.RiskWrite, true),
		)
		routes = append(routes,
			snapshotRoutes(kind)...,
		)
	}
	// QEMU-only suspend/resume.
	routes = append(routes,
		statusRoute("qemu", "suspend", plugin.RiskWrite, true),
		statusRoute("qemu", "resume", plugin.RiskWrite, false),
	)
	return routes
}

func statusRoute(kind, action string, risk plugin.RiskLevel, _ bool) plugin.Route {
	return plugin.Route{
		ID:         fmt.Sprintf("proxmox.%s.%s", kind, action),
		Method:     plugin.MethodPost,
		Path:       fmt.Sprintf("/nodes/{node}/%s/{vmid}/status/%s", kind, action),
		Permission: fmt.Sprintf("proxmox.%s.write", kind),
		Risk:       risk,
		AuditEvent: fmt.Sprintf("proxmox.%s.%s", kind, action),
		Handle:     guestStatus(kind, action),
	}
}

func snapshotRoutes(kind string) []plugin.Route {
	return []plugin.Route{
		{ID: fmt.Sprintf("proxmox.%s.snapshot.create", kind), Method: plugin.MethodPost, Path: fmt.Sprintf("/nodes/{node}/%s/{vmid}/snapshot", kind), Permission: fmt.Sprintf("proxmox.%s.snapshot", kind), Risk: plugin.RiskWrite, AuditEvent: fmt.Sprintf("proxmox.%s.snapshot.create", kind), Input: snapshotSchema(kind), Handle: snapshotCreate(kind)},
		{ID: fmt.Sprintf("proxmox.%s.snapshot.rollback", kind), Method: plugin.MethodPost, Path: fmt.Sprintf("/nodes/{node}/%s/{vmid}/snapshot/{snapname}/rollback", kind), Permission: fmt.Sprintf("proxmox.%s.snapshot", kind), Risk: plugin.RiskDestructive, AuditEvent: fmt.Sprintf("proxmox.%s.snapshot.rollback", kind), Handle: snapshotRollback(kind)},
		{ID: fmt.Sprintf("proxmox.%s.snapshot.delete", kind), Method: plugin.MethodDelete, Path: fmt.Sprintf("/nodes/{node}/%s/{vmid}/snapshot/{snapname}", kind), Permission: fmt.Sprintf("proxmox.%s.snapshot", kind), Risk: plugin.RiskDestructive, AuditEvent: fmt.Sprintf("proxmox.%s.snapshot.delete", kind), Handle: snapshotDelete(kind)},
	}
}

func pvePath(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return "/" + strings.Join(escaped, "/")
}

func requireNode(rc *plugin.RequestContext) (string, error) {
	node := rc.Param("node")
	if !validNode(node) {
		return "", fmt.Errorf("%w: invalid node", plugin.ErrInvalidInput)
	}
	return node, nil
}

func requireGuest(rc *plugin.RequestContext) (string, string, error) {
	node, err := requireNode(rc)
	if err != nil {
		return "", "", err
	}
	vmid := rc.Param("vmid")
	if !validVMID(vmid) {
		return "", "", fmt.Errorf("%w: invalid vmid", plugin.ErrInvalidInput)
	}
	return node, vmid, nil
}

func requireStorage(rc *plugin.RequestContext) (string, string, error) {
	node, err := requireNode(rc)
	if err != nil {
		return "", "", err
	}
	storage := rc.Param("storage")
	if !validStorage(storage) {
		return "", "", fmt.Errorf("%w: invalid storage", plugin.ErrInvalidInput)
	}
	return node, storage, nil
}

func requireSnapshot(rc *plugin.RequestContext) (string, string, string, error) {
	node, vmid, err := requireGuest(rc)
	if err != nil {
		return "", "", "", err
	}
	snapname := rc.Param("snapname")
	if !validSnapName(snapname) {
		return "", "", "", fmt.Errorf("%w: invalid snapshot name", plugin.ErrInvalidInput)
	}
	return node, vmid, snapname, nil
}

// --- Tree -----------------------------------------------------------------

func treeNodes(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	nodes, err := s.list(rc.Ctx, "/nodes")
	if err != nil {
		return nil, err
	}
	out := make([]plugin.TreeNode, 0, len(nodes))
	for _, n := range nodes {
		name := str(n["node"])
		if name == "" {
			continue
		}
		out = append(out, plugin.TreeNode{
			Key:          "node:" + name,
			Label:        name,
			Icon:         icon("server"),
			ResourceKind: "guest",
			ListParams:   map[string]string{"node": name},
			Leaf:         true,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: out, Total: ptr(len(out))}, nil
}

func treeStorage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	stores, err := s.list(rc.Ctx, "/cluster/resources?type=storage")
	if err != nil {
		return nil, err
	}
	out := make([]plugin.TreeNode, 0, len(stores))
	for _, st := range stores {
		node := str(st["node"])
		storage := str(st["storage"])
		if storage == "" {
			continue
		}
		out = append(out, plugin.TreeNode{
			Key:   "storage:" + node + ":" + storage,
			Label: storage + " (" + node + ")",
			Icon:  icon("database"),
			Ref:   &plugin.ResourceRef{Kind: "storage", Namespace: node, Name: storage, UID: storage},
			Leaf:  true,
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: out, Total: ptr(len(out))}, nil
}

// --- Lists ----------------------------------------------------------------

func listGuests(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		items, err := s.list(rc.Ctx, "/cluster/resources?type=vm")
		if err != nil {
			return nil, err
		}
		rows := make([]row, 0, len(items))
		onlyNode := rc.Query().Get("p.node")
		if onlyNode == "" {
			onlyNode = rc.Param("node")
		}
		for _, g := range items {
			guestKind := str(g["type"])
			if guestKind == "" {
				continue
			}
			if kind != "" && guestKind != kind {
				continue
			}
			node := str(g["node"])
			if onlyNode != "" && node != onlyNode {
				continue
			}
			vmid := str(g["vmid"])
			name := guestName(g, guestKind, vmid)
			rows = append(rows, row{
				"kindIcon": guestKindIcon(guestKind),
				"name":     name,
				"type":     guestKind,
				"mode":     guestMode(g),
				"template": isTemplateValue(g["template"]),
				"vmid":     numInt(g["vmid"]),
				"node":     node,
				"status":   str(g["status"]),
				"cpu":      round1(numFloat(g["cpu"]) * 100),
				"mem":      numInt(g["mem"]),
				"maxmem":   numInt(g["maxmem"]),
				"uptime":   numInt(g["uptime"]),
				"tags":     str(g["tags"]),
				"ref":      plugin.ResourceRef{Kind: guestKind, Namespace: node, Name: name, UID: vmid},
			})
		}
		return pageRows(rc, rows)
	}
}

func guestKindIcon(kind string) string {
	if kind == "lxc" {
		return "box"
	}
	return "monitor"
}

func guestMode(g row) string {
	if isTemplateValue(g["template"]) {
		return "Template"
	}
	return "Instance"
}

func isTemplateValue(v any) bool {
	switch strings.ToLower(str(v)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func listNodes(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, "/nodes")
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, n := range items {
		name := str(n["node"])
		rows = append(rows, row{
			"name":   name,
			"status": str(n["status"]),
			"cpu":    round1(numFloat(n["cpu"]) * 100),
			"mem":    numInt(n["mem"]),
			"maxmem": numInt(n["maxmem"]),
			"uptime": numInt(n["uptime"]),
			"ref":    plugin.ResourceRef{Kind: "node", Namespace: name, Name: name, UID: name},
		})
	}
	return pageRows(rc, rows)
}

func nodeOptions(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, "/nodes")
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	source := rc.Param("node")
	for _, n := range items {
		name := str(n["node"])
		if name == "" || name == source {
			continue
		}
		label := name
		if status := str(n["status"]); status != "" {
			label += " (" + status + ")"
		}
		rows = append(rows, row{
			"name":   name,
			"label":  label,
			"value":  name,
			"status": str(n["status"]),
		})
	}
	return pageRows(rc, rows)
}

func backupStorageOptions(rc *plugin.RequestContext) (any, error) {
	return storageOptions(rc, "backup")
}

func guestStorageOptions(rc *plugin.RequestContext) (any, error) {
	content := rc.Query().Get("content")
	if content == "" {
		content = rc.Param("content")
	}
	if content == "" {
		content = "images"
	}
	return storageOptions(rc, content)
}

func storageOptions(rc *plugin.RequestContext, requiredContent string) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, pvePath("nodes", node, "storage"))
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, st := range items {
		storage := str(st["storage"])
		if storage == "" || !validStorage(storage) || !storageSupportsContent(st, requiredContent) {
			continue
		}
		label := storage
		if typ := str(st["type"]); typ != "" {
			label += " (" + typ + ")"
		}
		rows = append(rows, row{
			"name":  storage,
			"label": label,
			"value": storage,
		})
	}
	return pageRows(rc, rows)
}

func storageSupportsContent(st row, content string) bool {
	for _, part := range strings.Split(str(st["content"]), ",") {
		if strings.TrimSpace(part) == content {
			return true
		}
	}
	return false
}

func listStorage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, "/cluster/resources?type=storage")
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, st := range items {
		node := str(st["node"])
		if r, ok := storageRow(node, st); ok {
			rows = append(rows, r)
		}
	}
	return pageRows(rc, rows)
}

func listNodeStorage(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, pvePath("nodes", node, "storage"))
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, st := range items {
		if r, ok := storageRow(node, st); ok {
			rows = append(rows, r)
		}
	}
	return pageRows(rc, rows)
}

func storageRow(node string, st row) (row, bool) {
	storage := str(st["storage"])
	if storage == "" {
		return nil, false
	}
	typ := str(st["plugintype"])
	if typ == "" {
		typ = str(st["type"])
	}
	used := numInt(st["disk"])
	if used == 0 {
		used = numInt(st["used"])
	}
	total := numInt(st["maxdisk"])
	if total == 0 {
		total = numInt(st["total"])
	}
	status := str(st["status"])
	if status == "" {
		status = nodeStorageStatus(st)
	}
	return row{
		"name":    storage,
		"node":    node,
		"type":    typ,
		"content": str(st["content"]),
		"usedPct": percent(used, total),
		"used":    used,
		"total":   total,
		"status":  status,
		"ref":     plugin.ResourceRef{Kind: "storage", Namespace: node, Name: storage, UID: storage},
	}, true
}

func percent(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return round1(float64(used) / float64(total) * 100)
}

func nodeStorageStatus(st row) string {
	if st["enabled"] != nil && numInt(st["enabled"]) == 0 {
		return "disabled"
	}
	if numInt(st["active"]) == 1 {
		return "online"
	}
	return "available"
}

func listStorageContent(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, storage, err := requireStorage(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, pvePath("nodes", node, "storage", storage, "content"))
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		volid := str(it["volid"])
		content := str(it["content"])
		rows = append(rows, row{
			"name":      volid,
			"content":   content,
			"guestType": backupGuestType(volid),
			"format":    str(it["format"]),
			"protected": rowBool(it["protected"]),
			"size":      numInt(it["size"]),
			"vmid":      str(it["vmid"]),
			"ctime":     rfcTime(it["ctime"]),
			"ref":       refForVolume("volume", node, storage, volid),
		})
	}
	return pageRows(rc, rows)
}

func listTasks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, pvePath("nodes", node, "tasks"))
	if err != nil {
		return nil, err
	}
	itemsWithNode := make([]row, 0, len(items))
	for _, t := range items {
		if str(t["node"]) == "" {
			t["node"] = node
		}
		itemsWithNode = append(itemsWithNode, t)
	}
	return pageRows(rc, taskRows(itemsWithNode))
}

func listClusterTasks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := s.list(rc.Ctx, "/cluster/tasks")
	if err != nil {
		return nil, err
	}
	return pageRows(rc, taskRows(items))
}

func taskRows(items []row) []row {
	rows := make([]row, 0, len(items))
	for _, t := range items {
		upid := str(t["upid"])
		node := str(t["node"])
		status := str(t["status"])
		if status == "" && t["endtime"] == nil {
			status = "running"
		} else if status == "" {
			status = str(t["exitstatus"])
		}
		rows = append(rows, row{
			"name":       str(t["type"]),
			"id":         str(t["id"]),
			"user":       str(t["user"]),
			"status":     status,
			"exitstatus": str(t["exitstatus"]),
			"starttime":  rfcTime(t["starttime"]),
			"endtime":    rfcTime(t["endtime"]),
			"_id":        upid,
			"ref":        plugin.ResourceRef{Kind: "task", Namespace: node, Name: node, UID: upid},
		})
	}
	return rows
}

func listSnapshots(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		items, err := s.list(rc.Ctx, pvePath("nodes", node, kind, vmid, "snapshot"))
		if err != nil {
			return nil, err
		}
		rows := make([]row, 0, len(items))
		for _, sn := range items {
			name := str(sn["name"])
			rows = append(rows, row{
				"name":        name,
				"description": str(sn["description"]),
				"vmstate":     rowBool(sn["vmstate"]),
				"snaptime":    rfcTime(sn["snaptime"]),
				"ref":         plugin.ResourceRef{Kind: "snapshot", Namespace: node, Name: vmid, UID: name},
			})
		}
		return pageRows(rc, rows)
	}
}

func listBackups(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, vmid, err := requireGuest(rc)
	if err != nil {
		return nil, err
	}
	stores, err := s.list(rc.Ctx, pvePath("nodes", node, "storage"))
	if err != nil {
		return nil, err
	}
	rows := []row{}
	for _, st := range stores {
		if !strings.Contains(str(st["content"]), "backup") {
			continue
		}
		storage := str(st["storage"])
		if !validStorage(storage) {
			continue
		}
		items, err := s.list(rc.Ctx, pvePath("nodes", node, "storage", storage, "content")+"?content=backup")
		if err != nil {
			continue
		}
		for _, it := range items {
			if vmid != "" && str(it["vmid"]) != vmid {
				continue
			}
			volid := str(it["volid"])
			guestType := backupGuestType(volid)
			rows = append(rows, row{
				"name":      volid,
				"content":   "backup",
				"guestType": guestType,
				"storage":   storage,
				"protected": rowBool(it["protected"]),
				"size":      numInt(it["size"]),
				"format":    str(it["format"]),
				"notes":     str(it["notes"]),
				"ctime":     rfcTime(it["ctime"]),
				"ref":       refForVolume("backup", node, storage, volid),
			})
		}
	}
	return pageRows(rc, rows)
}

// --- Documents ------------------------------------------------------------

func guestConfig(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		return s.object(rc.Ctx, pvePath("nodes", node, kind, vmid, "config"))
	}
}

func guestOverview(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		status, err := s.object(rc.Ctx, pvePath("nodes", node, kind, vmid, "status", "current"))
		if err != nil {
			return nil, err
		}
		cfg, err := s.object(rc.Ctx, pvePath("nodes", node, kind, vmid, "config"))
		if err != nil {
			return nil, err
		}
		cores := numInt(cfg["cores"])
		sockets := numInt(cfg["sockets"])
		cpuTotal := cores * sockets
		if cpuTotal == 0 {
			cpuTotal = cores
		}
		out := row{
			"node":             node,
			"vmid":             vmid,
			"name":             stringOr(str(cfg["name"]), str(cfg["hostname"])),
			"status":           str(status["status"]),
			"template":         isTemplateValue(cfg["template"]),
			"cpu":              round1(numFloat(status["cpu"]) * 100),
			"cpuTotal":         cpuTotal,
			"mem":              numInt(status["mem"]),
			"maxmem":           numInt(status["maxmem"]),
			"memPct":           memoryPercent(status["mem"], status["maxmem"]),
			"uptime":           numInt(status["uptime"]),
			"lock":             str(status["lock"]),
			"ha":               str(status["ha"]),
			"tags":             str(cfg["tags"]),
			"cores":            cores,
			"sockets":          sockets,
			"memory":           numInt(cfg["memory"]),
			"memoryConfigured": memoryMiBBytes(cfg["memory"]),
			"memoryMinimum":    memoryMiBBytes(cfg["balloon"]),
			"memoryCurrent":    memoryCurrentBytes(cfg["memory"]),
			"ostype":           str(cfg["ostype"]),
		}
		if out["name"] == "" {
			out["name"] = kind + "/" + vmid
		}
		return out, nil
	}
}

func nodeStatus(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	return s.object(rc.Ctx, pvePath("nodes", node, "status"))
}

// --- Actions --------------------------------------------------------------

func guestStatus(kind, action string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		path := pvePath("nodes", node, kind, vmid, "status", action)
		if err := s.post(rc.Ctx, path, nil); err != nil {
			return nil, err
		}
		return actionResult{OK: true}, nil
	}
}

func guestMigrate(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		var in struct {
			Target string `json:"target" validate:"required"`
			Online bool   `json:"online"`
		}
		if err := rc.Bind(&in); err != nil {
			return nil, err
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		if !validNode(in.Target) {
			return nil, fmt.Errorf("%w: invalid target node", plugin.ErrInvalidInput)
		}
		body := map[string]any{"target": in.Target}
		if in.Online {
			if kind == "lxc" {
				body["restart"] = 1
			} else {
				body["online"] = 1
			}
		}
		path := pvePath("nodes", node, kind, vmid, "migrate")
		if err := s.post(rc.Ctx, path, body); err != nil {
			return nil, err
		}
		return actionResult{OK: true}, nil
	}
}

func snapshotCreate(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		var in struct {
			Snapname    string `json:"snapname" validate:"required"`
			Description string `json:"description"`
			VMState     bool   `json:"vmstate"`
		}
		if err := rc.Bind(&in); err != nil {
			return nil, err
		}
		if !validSnapName(in.Snapname) {
			return nil, fmt.Errorf("%w: invalid snapshot name", plugin.ErrInvalidInput)
		}
		body := map[string]any{"snapname": in.Snapname}
		if in.Description != "" {
			body["description"] = in.Description
		}
		if in.VMState && kind == "qemu" {
			body["vmstate"] = 1
		}
		node, vmid, err := requireGuest(rc)
		if err != nil {
			return nil, err
		}
		path := pvePath("nodes", node, kind, vmid, "snapshot")
		if err := s.post(rc.Ctx, path, body); err != nil {
			return nil, err
		}
		return actionResult{OK: true}, nil
	}
}

func snapshotRollback(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, snapname, err := requireSnapshot(rc)
		if err != nil {
			return nil, err
		}
		path := pvePath("nodes", node, kind, vmid, "snapshot", snapname, "rollback")
		if err := s.post(rc.Ctx, path, nil); err != nil {
			return nil, err
		}
		return actionResult{OK: true}, nil
	}
}

func snapshotDelete(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid, snapname, err := requireSnapshot(rc)
		if err != nil {
			return nil, err
		}
		path := pvePath("nodes", node, kind, vmid, "snapshot", snapname)
		if err := s.del(rc.Ctx, path); err != nil {
			return nil, err
		}
		return actionResult{OK: true}, nil
	}
}

func backupCreate(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	var in struct {
		Storage  string `json:"storage" validate:"required"`
		Mode     string `json:"mode"`
		Compress string `json:"compress"`
	}
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	node, vmid, err := requireGuest(rc)
	if err != nil {
		return nil, err
	}
	if !validStorage(in.Storage) {
		return nil, fmt.Errorf("%w: invalid storage", plugin.ErrInvalidInput)
	}
	body := map[string]any{
		"vmid":     vmid,
		"storage":  in.Storage,
		"mode":     stringOr(in.Mode, "snapshot"),
		"compress": stringOr(in.Compress, "zstd"),
	}
	if !validBackupMode(str(body["mode"])) {
		return nil, fmt.Errorf("%w: invalid backup mode", plugin.ErrInvalidInput)
	}
	if !validCompression(str(body["compress"])) {
		return nil, fmt.Errorf("%w: invalid compression", plugin.ErrInvalidInput)
	}
	if err := s.post(rc.Ctx, pvePath("nodes", node, "vzdump"), body); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

func backupDelete(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, storage, err := requireStorage(rc)
	if err != nil {
		return nil, err
	}
	volume := rc.Param("volume")
	if !validBackupVolume(storage, volume) {
		return nil, fmt.Errorf("%w: invalid backup volume", plugin.ErrInvalidInput)
	}
	path := pvePath("nodes", node, "storage", storage, "content", volume)
	if err := s.del(rc.Ctx, path); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

// --- Input schemas --------------------------------------------------------

func snapshotSchema(kind string) *plugin.Schema {
	fields := []plugin.Field{
		{Key: "snapname", Label: "Name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: snapRe.String(), Message: "letters, digits, dash and underscore only"}}},
		{Key: "description", Label: "Description", Type: plugin.FieldTextarea},
	}
	if kind == "qemu" {
		fields = append(fields, plugin.Field{Key: "vmstate", Label: "Include RAM state", Type: plugin.FieldToggle})
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Snapshot", Fields: fields}}}
}

func migrateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Migrate", Fields: []plugin.Field{
		{Key: "target", Label: "Target node", Type: plugin.FieldSelect, Required: true, OptionsSource: &plugin.DataSource{RouteID: "proxmox.node.options", Params: map[string]string{"node": "${resource.namespace}"}}},
		{Key: "online", Label: "Online migration", Type: plugin.FieldToggle, Default: true, Help: "Keep a running VM online when possible. Containers use restart migration."},
	}}}}
}

func backupSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Backup", Fields: []plugin.Field{
		{Key: "storage", Label: "Storage", Type: plugin.FieldSelect, Required: true, OptionsSource: &plugin.DataSource{RouteID: "proxmox.node.backup_storage.options", Params: map[string]string{"node": "${resource.namespace}"}}},
		{Key: "mode", Label: "Mode", Type: plugin.FieldSelect, Default: "snapshot", Options: []plugin.Option{
			{Label: "Snapshot", Value: "snapshot"},
			{Label: "Suspend", Value: "suspend"},
			{Label: "Stop", Value: "stop"},
		}},
		{Key: "compress", Label: "Compression", Type: plugin.FieldSelect, Default: "zstd", Options: []plugin.Option{
			{Label: "Zstd", Value: "zstd"},
			{Label: "LZO", Value: "lzo"},
			{Label: "GZIP", Value: "gzip"},
			{Label: "None", Value: "0"},
		}},
	}}}}
}

// --- Helpers --------------------------------------------------------------

func guestName(g row, kind, vmid string) string {
	if name := str(g["name"]); name != "" {
		return name
	}
	return kind + "/" + vmid
}

func backupGuestType(volid string) string {
	switch {
	case strings.Contains(volid, "vzdump-qemu-"):
		return "qemu"
	case strings.Contains(volid, "vzdump-lxc-"):
		return "lxc"
	default:
		return ""
	}
}

func memoryMiBBytes(v any) int64 {
	current, maximum := memoryMiB(v)
	if maximum > 0 {
		return maximum * 1024 * 1024
	}
	return current * 1024 * 1024
}

func memoryCurrentBytes(v any) int64 {
	current, maximum := memoryMiB(v)
	if current > 0 {
		return current * 1024 * 1024
	}
	return maximum * 1024 * 1024
}

func memoryMiB(v any) (current int64, maximum int64) {
	text := strings.TrimSpace(str(v))
	if text == "" {
		return 0, 0
	}
	for _, part := range strings.Split(text, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "current=") {
			current = numInt(strings.TrimPrefix(part, "current="))
			continue
		}
		if maximum == 0 {
			maximum = numInt(part)
		}
	}
	if current == 0 {
		current = maximum
	}
	return current, maximum
}

func memoryPercent(used, maximum any) float64 {
	limit := numFloat(maximum)
	if limit <= 0 {
		return 0
	}
	return round1(numFloat(used) / limit * 100)
}

func refForVolume(kind, node, storage, volid string) plugin.ResourceRef {
	return plugin.ResourceRef{Kind: kind, Namespace: node, Name: storage, UID: volid}
}

func icon(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

func rfcTime(v any) string {
	sec := numInt(v)
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func stringOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
