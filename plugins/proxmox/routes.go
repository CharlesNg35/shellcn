package proxmox

import (
	"fmt"
	"strings"
	"time"

	"github.com/charlesng/shellcn/internal/plugin"
)

type actionResult struct {
	OK bool `json:"ok"`
}

func Routes() []plugin.Route {
	routes := []plugin.Route{
		// Tree.
		{ID: "proxmox.tree.nodes", Method: plugin.MethodGet, Path: "/tree/nodes", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.tree.nodes", Handle: treeNodes},
		{ID: "proxmox.tree.node", Method: plugin.MethodGet, Path: "/tree/nodes/{node}", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.tree.node", Handle: treeNodeChildren},
		{ID: "proxmox.tree.storage", Method: plugin.MethodGet, Path: "/tree/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.tree.storage", Handle: treeStorage},

		// Lists.
		{ID: "proxmox.qemu.list", Method: plugin.MethodGet, Path: "/qemu", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.list", Handle: listGuests("qemu")},
		{ID: "proxmox.lxc.list", Method: plugin.MethodGet, Path: "/lxc", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.list", Handle: listGuests("lxc")},
		{ID: "proxmox.node.list", Method: plugin.MethodGet, Path: "/nodes", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.list", Handle: listNodes},
		{ID: "proxmox.storage.list", Method: plugin.MethodGet, Path: "/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.storage.list", Handle: listStorage},
		{ID: "proxmox.node.storage", Method: plugin.MethodGet, Path: "/nodes/{node}/storage", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.storage", Handle: listNodeStorage},
		{ID: "proxmox.node.tasks", Method: plugin.MethodGet, Path: "/nodes/{node}/tasks", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.node.tasks", Handle: listTasks},
		{ID: "proxmox.storage.content", Method: plugin.MethodGet, Path: "/nodes/{node}/storage/{storage}/content", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.storage.content", Handle: listStorageContent},
		{ID: "proxmox.qemu.snapshots", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/snapshot", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.snapshots", Handle: listSnapshots("qemu")},
		{ID: "proxmox.lxc.snapshots", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/snapshot", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.snapshots", Handle: listSnapshots("lxc")},
		{ID: "proxmox.qemu.backups", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/backups", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.backups", Handle: listBackups},
		{ID: "proxmox.lxc.backups", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/backups", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.backups", Handle: listBackups},

		// Documents.
		{ID: "proxmox.qemu.config", Method: plugin.MethodGet, Path: "/nodes/{node}/qemu/{vmid}/config", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.qemu.config", Handle: guestConfig("qemu")},
		{ID: "proxmox.lxc.config", Method: plugin.MethodGet, Path: "/nodes/{node}/lxc/{vmid}/config", Permission: "proxmox.read", Risk: plugin.RiskSafe, AuditEvent: "proxmox.lxc.config", Handle: guestConfig("lxc")},
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
			Key:            "node:" + name,
			Label:          name,
			Icon:           icon("server"),
			Ref:            &plugin.ResourceRef{Kind: "node", Namespace: name, Name: name, UID: name},
			ChildrenSource: &plugin.DataSource{RouteID: "proxmox.tree.node", Params: map[string]string{"node": name}},
		})
	}
	return plugin.Page[plugin.TreeNode]{Items: out, Total: ptr(len(out))}, nil
}

func treeNodeChildren(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node := rc.Param("node")
	guests, err := s.list(rc.Ctx, "/cluster/resources?type=vm")
	if err != nil {
		return nil, err
	}
	out := []plugin.TreeNode{}
	for _, g := range guests {
		if str(g["node"]) != node {
			continue
		}
		kind := str(g["type"])
		vmid := str(g["vmid"])
		name := guestName(g, kind, vmid)
		out = append(out, plugin.TreeNode{
			Key:   kind + ":" + node + ":" + vmid,
			Label: name,
			Icon:  guestIcon(kind),
			Ref:   &plugin.ResourceRef{Kind: kind, Namespace: node, Name: name, UID: vmid},
			Leaf:  true,
			Badge: &plugin.Badge{Value: str(g["status"]), Severity: statusSeverity(str(g["status"]))},
			Data:  g,
		})
	}
	stores, err := s.list(rc.Ctx, "/nodes/"+node+"/storage")
	if err == nil {
		for _, st := range stores {
			storage := str(st["storage"])
			out = append(out, plugin.TreeNode{
				Key:   "storage:" + node + ":" + storage,
				Label: storage,
				Icon:  icon("database"),
				Ref:   &plugin.ResourceRef{Kind: "storage", Namespace: node, Name: storage, UID: storage},
				Leaf:  true,
			})
		}
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
		for _, g := range items {
			if str(g["type"]) != kind {
				continue
			}
			node := str(g["node"])
			vmid := str(g["vmid"])
			name := guestName(g, kind, vmid)
			rows = append(rows, row{
				"name":   name,
				"vmid":   numInt(g["vmid"]),
				"node":   node,
				"status": str(g["status"]),
				"cpu":    round1(numFloat(g["cpu"]) * 100),
				"mem":    numInt(g["mem"]),
				"maxmem": numInt(g["maxmem"]),
				"uptime": numInt(g["uptime"]),
				"tags":   str(g["tags"]),
				"ref":    plugin.ResourceRef{Kind: kind, Namespace: node, Name: name, UID: vmid},
			})
		}
		return pageRows(rc, rows)
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
	node := rc.Param("node")
	items, err := s.list(rc.Ctx, "/nodes/"+node+"/storage")
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
		"used":    used,
		"total":   total,
		"status":  status,
		"ref":     plugin.ResourceRef{Kind: "storage", Namespace: node, Name: storage, UID: storage},
	}, true
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
	node, storage := rc.Param("node"), rc.Param("storage")
	items, err := s.list(rc.Ctx, fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage))
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		volid := str(it["volid"])
		rows = append(rows, row{
			"name":    volid,
			"content": str(it["content"]),
			"format":  str(it["format"]),
			"size":    numInt(it["size"]),
			"vmid":    str(it["vmid"]),
			"ctime":   rfcTime(it["ctime"]),
			"ref":     refForVolume("volume", node, storage, volid),
		})
	}
	return pageRows(rc, rows)
}

func listTasks(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node := rc.Param("node")
	items, err := s.list(rc.Ctx, "/nodes/"+node+"/tasks")
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(items))
	for _, t := range items {
		upid := str(t["upid"])
		status := str(t["status"])
		if status == "" && t["endtime"] == nil {
			status = "running"
		}
		rows = append(rows, row{
			"name":      str(t["type"]),
			"id":        str(t["id"]),
			"user":      str(t["user"]),
			"status":    status,
			"starttime": rfcTime(t["starttime"]),
			"ref":       plugin.ResourceRef{Kind: "task", Namespace: node, Name: str(t["type"]), UID: upid},
		})
	}
	return pageRows(rc, rows)
}

func listSnapshots(kind string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		node, vmid := rc.Param("node"), rc.Param("vmid")
		items, err := s.list(rc.Ctx, fmt.Sprintf("/nodes/%s/%s/%s/snapshot", node, kind, vmid))
		if err != nil {
			return nil, err
		}
		rows := make([]row, 0, len(items))
		for _, sn := range items {
			name := str(sn["name"])
			rows = append(rows, row{
				"name":        name,
				"description": str(sn["description"]),
				"parent":      str(sn["parent"]),
				"snaptime":    rfcTime(sn["snaptime"]),
				// Pack node/vmid/snapname across the ref so row actions resolve them.
				"ref": plugin.ResourceRef{Kind: "snapshot", Namespace: node, Name: vmid, UID: name},
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
	node, vmid := rc.Param("node"), rc.Param("vmid")
	stores, err := s.list(rc.Ctx, "/nodes/"+node+"/storage")
	if err != nil {
		return nil, err
	}
	rows := []row{}
	for _, st := range stores {
		if !strings.Contains(str(st["content"]), "backup") {
			continue
		}
		storage := str(st["storage"])
		items, err := s.list(rc.Ctx, fmt.Sprintf("/nodes/%s/storage/%s/content?content=backup", node, storage))
		if err != nil {
			continue
		}
		for _, it := range items {
			if vmid != "" && str(it["vmid"]) != vmid {
				continue
			}
			volid := str(it["volid"])
			rows = append(rows, row{
				"name":    volid,
				"storage": storage,
				"size":    numInt(it["size"]),
				"format":  str(it["format"]),
				"notes":   str(it["notes"]),
				"ctime":   rfcTime(it["ctime"]),
				"ref":     refForVolume("backup", node, storage, volid),
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
		return s.object(rc.Ctx, fmt.Sprintf("/nodes/%s/%s/%s/config", rc.Param("node"), kind, rc.Param("vmid")))
	}
}

func nodeStatus(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	return s.object(rc.Ctx, "/nodes/"+rc.Param("node")+"/status")
}

// --- Actions --------------------------------------------------------------

func guestStatus(kind, action string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		s, err := sess(rc)
		if err != nil {
			return nil, err
		}
		path := fmt.Sprintf("/nodes/%s/%s/%s/status/%s", rc.Param("node"), kind, rc.Param("vmid"), action)
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
		body := map[string]any{"target": in.Target}
		if in.Online {
			if kind == "lxc" {
				body["restart"] = 1
			} else {
				body["online"] = 1
			}
		}
		path := fmt.Sprintf("/nodes/%s/%s/%s/migrate", rc.Param("node"), kind, rc.Param("vmid"))
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
		body := map[string]any{"snapname": in.Snapname}
		if in.Description != "" {
			body["description"] = in.Description
		}
		if in.VMState && kind == "qemu" {
			body["vmstate"] = 1
		}
		path := fmt.Sprintf("/nodes/%s/%s/%s/snapshot", rc.Param("node"), kind, rc.Param("vmid"))
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
		path := fmt.Sprintf("/nodes/%s/%s/%s/snapshot/%s/rollback", rc.Param("node"), kind, rc.Param("vmid"), rc.Param("snapname"))
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
		path := fmt.Sprintf("/nodes/%s/%s/%s/snapshot/%s", rc.Param("node"), kind, rc.Param("vmid"), rc.Param("snapname"))
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
	body := map[string]any{
		"vmid":     rc.Param("vmid"),
		"storage":  in.Storage,
		"mode":     stringOr(in.Mode, "snapshot"),
		"compress": stringOr(in.Compress, "zstd"),
	}
	if err := s.post(rc.Ctx, "/nodes/"+rc.Param("node")+"/vzdump", body); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

func backupDelete(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", rc.Param("node"), rc.Param("storage"), rc.Param("volume"))
	if err := s.del(rc.Ctx, path); err != nil {
		return nil, err
	}
	return actionResult{OK: true}, nil
}

// --- Input schemas --------------------------------------------------------

func snapshotSchema(kind string) *plugin.Schema {
	fields := []plugin.Field{
		{Key: "snapname", Label: "Name", Type: plugin.FieldText, Required: true, Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[A-Za-z][A-Za-z0-9_-]+$`, Message: "letters, digits, dash and underscore only"}}},
		{Key: "description", Label: "Description", Type: plugin.FieldTextarea},
	}
	if kind == "qemu" {
		fields = append(fields, plugin.Field{Key: "vmstate", Label: "Include RAM state", Type: plugin.FieldToggle})
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Snapshot", Fields: fields}}}
}

func migrateSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Migrate", Fields: []plugin.Field{
		{Key: "target", Label: "Target node", Type: plugin.FieldText, Required: true},
		{Key: "online", Label: "Live / online migration", Type: plugin.FieldToggle},
	}}}}
}

func backupSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Backup", Fields: []plugin.Field{
		{Key: "storage", Label: "Storage", Type: plugin.FieldText, Required: true},
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

func refForVolume(kind, node, storage, volid string) plugin.ResourceRef {
	return plugin.ResourceRef{Kind: kind, Namespace: node, Name: storage, UID: volid}
}

func icon(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

func guestIcon(kind string) plugin.Icon {
	if kind == "lxc" {
		return icon("box")
	}
	return icon("monitor")
}

func statusSeverity(status string) plugin.Severity {
	switch status {
	case "running", "online":
		return plugin.SeveritySuccess
	case "stopped", "offline":
		return plugin.SeveritySecondary
	default:
		return plugin.SeverityInfo
	}
}

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
