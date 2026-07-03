package proxmox

import "github.com/charlesng35/shellcn/sdk/plugin"

func haResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "ha",
		Title:   "HA Resources",
		List:    plugin.DataSource{RouteID: "proxmox.ha.resources"},
		Columns: haColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "state", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "detail", Label: "Detail", Icon: icon("shield-check"), Type: plugin.PanelObjectDetail, Config: genericInventoryDetail()},
			},
		},
	}
}

func poolResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "pool",
		Title:   "Pools",
		List:    plugin.DataSource{RouteID: "proxmox.pools"},
		Columns: poolColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}"},
			Tabs: []plugin.Panel{
				{Key: "detail", Label: "Detail", Icon: icon("folder-kanban"), Type: plugin.PanelObjectDetail, Config: genericInventoryDetail()},
			},
		},
	}
}

func replicationResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "replication",
		Title:   "Replication",
		List:    plugin.DataSource{RouteID: "proxmox.replication"},
		Columns: replicationColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "state", Severities: statusSeverities},
			Tabs: []plugin.Panel{
				{Key: "detail", Label: "Detail", Icon: icon("copy-check"), Type: plugin.PanelObjectDetail, Config: genericInventoryDetail()},
			},
		},
	}
}

func genericInventoryDetail() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{RawToggle: true}
}

func haColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Resource", Sortable: true},
		{Key: "type", Label: "Type", Sortable: true},
		{Key: "group", Label: "Group", Sortable: true},
		{Key: "state", Label: "State", Type: plugin.ColumnBadge, Sortable: true, Severities: statusSeverities},
		{Key: "node", Label: "Node", Sortable: true},
		{Key: "max_restart", Label: "Max restart", Type: plugin.ColumnNumber},
		{Key: "max_relocate", Label: "Max relocate", Type: plugin.ColumnNumber},
	}
}

func poolColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Pool", Sortable: true},
		{Key: "comment", Label: "Comment"},
		{Key: "members", Label: "Members", Type: plugin.ColumnNumber, Sortable: true},
	}
}

func replicationColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "name", Label: "Job", Sortable: true},
		{Key: "guest", Label: "Guest", Sortable: true},
		{Key: "source", Label: "Source", Sortable: true},
		{Key: "target", Label: "Target", Sortable: true},
		{Key: "schedule", Label: "Schedule"},
		{Key: "state", Label: "State", Type: plugin.ColumnBadge, Sortable: true, Severities: statusSeverities},
		{Key: "last_sync", Label: "Last sync", Type: plugin.ColumnDateTime, Sortable: true},
	}
}

func logColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "n", Label: "#", Type: plugin.ColumnNumber, Width: "5rem"},
		{Key: "time", Label: "Time", Type: plugin.ColumnDateTime, Sortable: true},
		{Key: "pri", Label: "Priority", Sortable: true},
		{Key: "t", Label: "Message"},
	}
}

func updateColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "Package", Label: "Package", Sortable: true},
		{Key: "CurrentState", Label: "Current", Sortable: true},
		{Key: "Version", Label: "Version", Sortable: true},
		{Key: "OldVersion", Label: "Old version", Sortable: true},
		{Key: "Description", Label: "Description"},
	}
}

func nodeStatusConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{
		Sections: []plugin.ObjectDetailSection{
			{Title: "Node", Fields: []plugin.ObjectDetailField{
				{Key: "node", Label: "Node"},
				{Key: "pveversion", Label: "PVE version"},
				{Key: "kversion", Label: "Kernel"},
				{Key: "uptime", Label: "Uptime", Type: plugin.ColumnNumber},
			}},
			{Title: "Runtime", Fields: []plugin.ObjectDetailField{
				{Key: "cpu", Label: "CPU usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "cpu", WarnAt: 75, CriticalAt: 90}},
				{Key: "mem", Label: "Memory usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "memPct", UsedKey: "memUsed", TotalKey: "memTotal", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 95}},
				{Key: "rootfs", Label: "Root filesystem", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "rootfsPct", UsedKey: "rootfsUsed", TotalKey: "rootfsTotal", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 90}},
			}},
		},
		RawToggle: true,
	}
}

func listHAResources(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := optionalList(rc.Ctx, s, "/cluster/ha/resources")
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(items))
	for _, it := range items {
		sid := stringOr(str(it["sid"]), str(it["id"]))
		row := cloneRow(it)
		row["name"] = sid
		row["type"] = str(it["type"])
		row["ref"] = plugin.ResourceIdentity{Kind: "ha", Name: sid, UID: sid}
		rows = append(rows, row)
	}
	return pageRows(rc, rows)
}

func listPools(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := optionalList(rc.Ctx, s, "/pools")
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(items))
	for _, it := range items {
		poolid := stringOr(str(it["poolid"]), str(it["id"]))
		row := cloneRow(it)
		row["name"] = poolid
		row["members"] = len(rowSlice(it["members"]))
		row["ref"] = plugin.ResourceIdentity{Kind: "pool", Name: poolid, UID: poolid}
		rows = append(rows, row)
	}
	return pageRows(rc, rows)
}

func listReplication(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	items, err := optionalList(rc.Ctx, s, "/cluster/replication")
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(items))
	for _, it := range items {
		id := stringOr(str(it["id"]), str(it["guest"]))
		row := cloneRow(it)
		row["name"] = id
		row["last_sync"] = rfcTime(it["last_sync"])
		row["ref"] = plugin.ResourceIdentity{Kind: "replication", Name: id, UID: id}
		rows = append(rows, row)
	}
	return pageRows(rc, rows)
}

func nodeSyslog(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	items, err := optionalList(rc.Ctx, s, pvePath("nodes", node, "syslog"))
	if err != nil {
		return nil, err
	}
	return pageRows(rc, logRows(items))
}

func nodeUpdates(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	node, err := requireNode(rc)
	if err != nil {
		return nil, err
	}
	items, err := optionalList(rc.Ctx, s, pvePath("nodes", node, "apt", "update"))
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(items))
	for _, item := range items {
		row := cloneRow(item)
		name := stringOr(str(item["Package"]), str(item["package"]))
		row["ref"] = plugin.ResourceIdentity{Kind: "package", Namespace: node, Name: name, UID: name}
		rows = append(rows, row)
	}
	return pageRows(rc, rows)
}

func logRows(items []plugin.TableRow) []plugin.TableRow {
	rows := make([]plugin.TableRow, 0, len(items))
	for _, it := range items {
		row := plugin.TableRow{
			"n":   numInt(it["n"]),
			"pri": str(it["pri"]),
			"t":   str(it["t"]),
		}
		if row["t"] == "" {
			row["t"] = str(it["msg"])
		}
		if row["time"] = rfcTime(it["time"]); row["time"] == "" {
			row["time"] = rfcTime(it["time_t"])
		}
		rows = append(rows, row)
	}
	return rows
}

func cloneRow(in plugin.TableRow) plugin.TableRow {
	out := make(plugin.TableRow, len(in)+1)
	for key, val := range in {
		out[key] = val
	}
	return out
}

func rowSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []plugin.TableRow:
		out := make([]any, len(t))
		for i := range t {
			out[i] = t[i]
		}
		return out
	default:
		return nil
	}
}

func nodeStatusRow(status plugin.TableRow) plugin.TableRow {
	row := cloneRow(status)
	if mem, ok := status["memory"].(map[string]any); ok {
		row["memUsed"] = numInt(mem["used"])
		row["memTotal"] = numInt(mem["total"])
		row["memPct"] = percent(numInt(mem["used"]), numInt(mem["total"]))
	}
	if rootfs, ok := status["rootfs"].(map[string]any); ok {
		row["rootfsUsed"] = numInt(rootfs["used"])
		row["rootfsTotal"] = numInt(rootfs["total"])
		row["rootfsPct"] = percent(numInt(rootfs["used"]), numInt(rootfs["total"]))
	}
	if numFloat(row["cpu"]) <= 1 {
		row["cpu"] = round1(numFloat(row["cpu"]) * 100)
	}
	return row
}
