package proxmox

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func overviewList(rc *plugin.RequestContext) (any, error) {
	row := plugin.TableRow{
		"name": "Overview",
		"ref":  plugin.ResourceIdentity{Kind: "overview", Name: "Overview", UID: "overview"},
	}
	return pageRows(rc, []plugin.TableRow{row})
}

func overviewMetricsConfig() plugin.MetricsConfig {
	return plugin.MetricsConfig{
		Stats: []plugin.MetricStat{
			{Key: "nodes", Label: "Nodes"},
			{Key: "guests", Label: "Guests"},
			{Key: "runningGuests", Label: "Running"},
			{Key: "failedTasks", Label: "Failed tasks"},
		},
		Usage: []plugin.MetricUsage{
			{Key: "cpu", Label: "CPU usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "cpu", WarnAt: 75, CriticalAt: 90}},
			{Key: "mem", Label: "Memory usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "mem", UsedKey: "memUsed", TotalKey: "memTotal", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 95}},
			{Key: "storage", Label: "Storage usage", Type: plugin.ColumnPercent, Usage: &plugin.UsageSpec{PercentKey: "storage", UsedKey: "storageUsed", TotalKey: "storageTotal", UsedType: plugin.ColumnBytes, TotalType: plugin.ColumnBytes, WarnAt: 80, CriticalAt: 90}},
		},
		Series: []plugin.MetricSeries{
			{Key: "cpu", Label: "CPU", Unit: "%"},
			{Key: "mem", Label: "Memory", Unit: "%"},
			{Key: "storage", Label: "Storage", Unit: "%"},
		},
		History: 60,
	}
}

func overviewFrame(ctx context.Context, s *Session) (plugin.TableRow, error) {
	nodes, err := nodeRows(ctx, s)
	if err != nil {
		return nil, err
	}
	guests, err := guestRows(ctx, s, "", "")
	if err != nil {
		return nil, err
	}
	storage, err := storageRows(ctx, s, "")
	if err != nil {
		return nil, err
	}
	tasks, err := optionalList(ctx, s, "/cluster/tasks")
	if err != nil {
		return nil, err
	}

	var cpuTotal, memUsed, memTotal, storageUsed, storageTotal float64
	onlineNodes := int64(0)
	for _, node := range nodes {
		if str(node["status"]) == "online" {
			onlineNodes++
		}
		cpuTotal += numFloat(node["cpu"])
		memUsed += numFloat(node["mem"])
		memTotal += numFloat(node["maxmem"])
	}
	runningGuests := int64(0)
	for _, guest := range guests {
		if str(guest["status"]) == "running" {
			runningGuests++
		}
	}
	for _, st := range storage {
		storageUsed += numFloat(st["used"])
		storageTotal += numFloat(st["total"])
	}
	failedTasks := int64(0)
	now := time.Now()
	for _, task := range tasks {
		if taskFailed(task) && withinHours(task["endtime"], now, 24) {
			failedTasks++
		}
	}

	cpuPct := 0.0
	if len(nodes) > 0 {
		cpuPct = round1(cpuTotal / float64(len(nodes)))
	}
	return plugin.TableRow{
		"nodes":         len(nodes),
		"onlineNodes":   onlineNodes,
		"guests":        len(guests),
		"runningGuests": runningGuests,
		"failedTasks":   failedTasks,
		"cpu":           cpuPct,
		"mem":           percent(int64(memUsed), int64(memTotal)),
		"memUsed":       int64(memUsed),
		"memTotal":      int64(memTotal),
		"storage":       percent(int64(storageUsed), int64(storageTotal)),
		"storageUsed":   int64(storageUsed),
		"storageTotal":  int64(storageTotal),
	}, nil
}

func healthColumns() []plugin.Column {
	return []plugin.Column{
		{Key: "severity", Label: "Severity", Type: plugin.ColumnBadge, Sortable: true, Severities: map[string]plugin.Severity{"critical": plugin.SeverityDanger, "warning": plugin.SeverityWarn, "info": plugin.SeverityInfo}},
		{Key: "title", Label: "Finding", Sortable: true},
		{Key: "resource", Label: "Resource", Sortable: true},
		{Key: "detail", Label: "Detail"},
		{Key: "recommendation", Label: "Recommendation"},
	}
}

func healthResource() plugin.ResourceType {
	return plugin.ResourceType{
		Kind:    "health",
		Title:   "Health",
		List:    plugin.DataSource{RouteID: "proxmox.health.list"},
		Columns: healthColumns(),
		Detail: plugin.DetailView{
			Header: plugin.HeaderSpec{Title: "${resource.name}", StatusField: "severity", Severities: map[string]plugin.Severity{"critical": plugin.SeverityDanger, "warning": plugin.SeverityWarn, "info": plugin.SeverityInfo}},
			Tabs: []plugin.Panel{
				{Key: "detail", Label: "Detail", Icon: icon("circle-alert"), Type: plugin.PanelObjectDetail, Config: plugin.ObjectDetailConfig{Sections: []plugin.ObjectDetailSection{{Title: "Finding", Fields: []plugin.ObjectDetailField{
					{Key: "severity", Label: "Severity", Type: plugin.ColumnBadge, Severities: map[string]plugin.Severity{"critical": plugin.SeverityDanger, "warning": plugin.SeverityWarn, "info": plugin.SeverityInfo}},
					{Key: "title", Label: "Finding"},
					{Key: "resource", Label: "Resource"},
					{Key: "detail", Label: "Detail"},
					{Key: "recommendation", Label: "Recommendation"},
				}}}, RawToggle: true}},
			},
		},
	}
}

func healthList(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	rows, err := healthRows(rc.Ctx, s)
	if err != nil {
		return nil, err
	}
	return pageRows(rc, rows)
}

func healthRows(ctx context.Context, s *Session) ([]plugin.TableRow, error) {
	nodes, err := nodeRows(ctx, s)
	if err != nil {
		return nil, err
	}
	guests, err := guestRows(ctx, s, "", "")
	if err != nil {
		return nil, err
	}
	storage, err := storageRows(ctx, s, "")
	if err != nil {
		return nil, err
	}
	tasks, err := optionalList(ctx, s, "/cluster/tasks")
	if err != nil {
		return nil, err
	}

	rows := make([]plugin.TableRow, 0)
	for _, node := range nodes {
		name := str(node["name"])
		if str(node["status"]) != "online" {
			rows = append(rows, healthRow("critical", "Node is not online", name, "Status is "+str(node["status"]), "Check node power, network, and cluster membership."))
		}
		if numFloat(node["cpu"]) >= 90 {
			rows = append(rows, healthRow("warning", "Node CPU pressure", name, "CPU usage is "+str(node["cpu"])+"%.", "Review guest placement or migrate workloads."))
		}
		if percent(numInt(node["mem"]), numInt(node["maxmem"])) >= 90 {
			rows = append(rows, healthRow("warning", "Node memory pressure", name, "Memory usage is above 90%.", "Review ballooning, limits, or workload placement."))
		}
	}
	for _, st := range storage {
		if numFloat(st["usedPct"]) >= 90 {
			rows = append(rows, healthRow("critical", "Storage nearly full", str(st["node"])+"/"+str(st["name"]), "Storage usage is "+str(st["usedPct"])+"%.", "Free space, prune backups, or expand storage."))
		} else if numFloat(st["usedPct"]) >= 80 {
			rows = append(rows, healthRow("warning", "Storage usage high", str(st["node"])+"/"+str(st["name"]), "Storage usage is "+str(st["usedPct"])+"%.", "Plan capacity before guests or backups fail."))
		}
		if status := str(st["status"]); status != "" && status != "online" && status != "available" {
			rows = append(rows, healthRow("warning", "Storage not online", str(st["node"])+"/"+str(st["name"]), "Status is "+status+".", "Check storage activation and backend health."))
		}
	}
	now := time.Now()
	for _, task := range tasks {
		if taskFailed(task) && withinHours(task["endtime"], now, 24) {
			rows = append(rows, healthRow("warning", "Recent task failed", taskResourceName(task), taskSummary(task), "Open the task log and fix the failed operation."))
		}
	}
	for _, guest := range guests {
		if str(guest["status"]) == "unknown" {
			rows = append(rows, healthRow("warning", "Guest status unknown", str(guest["node"])+"/"+str(guest["name"]), "The guest status is unknown.", "Check the guest and node task logs."))
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return healthRank(str(rows[i]["severity"])) > healthRank(str(rows[j]["severity"]))
	})
	return rows, nil
}

func healthRow(severity, title, resource, detail, recommendation string) plugin.TableRow {
	uid := strings.Join([]string{severity, title, resource, detail}, ":")
	return plugin.TableRow{
		"severity":       severity,
		"title":          title,
		"resource":       resource,
		"detail":         detail,
		"recommendation": recommendation,
		"ref":            plugin.ResourceIdentity{Kind: "health", Name: title, UID: uid},
	}
}

func healthRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 3
	case "warning":
		return 2
	default:
		return 1
	}
}

func taskTimelineConfig(watch *plugin.DataSource) plugin.TimelineConfig {
	return plugin.TimelineConfig{
		TimestampField: "time",
		TitleField:     "title",
		BodyField:      "body",
		SeverityField:  "severity",
		IconField:      "icon",
		EmptyText:      "No recent Proxmox tasks.",
		Watch:          watch,
	}
}

func taskTimeline(rc *plugin.RequestContext) (any, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, err
	}
	tasks, err := s.list(rc.Ctx, "/cluster/tasks")
	if err != nil {
		return nil, err
	}
	rows := make([]plugin.TableRow, 0, len(tasks))
	for _, task := range taskRows(tasks) {
		rows = append(rows, taskTimelineRow(task))
	}
	sortTimeline(rows)
	return pageRows(rc, rows)
}

func taskTimelineRow(task plugin.TableRow) plugin.TableRow {
	severity := "info"
	iconName := "loader"
	if str(task["status"]) == "running" {
		severity = "info"
	} else if str(task["exitstatus"]) == "OK" {
		severity = "success"
		iconName = "check"
	} else if str(task["exitstatus"]) != "" {
		severity = "danger"
		iconName = "circle-alert"
	}
	ref, _ := task["ref"].(plugin.ResourceIdentity)
	return plugin.TableRow{
		"time":     stringOr(str(task["endtime"]), str(task["starttime"])),
		"title":    taskSummary(task),
		"body":     "User " + str(task["user"]) + " on " + ref.Namespace,
		"severity": severity,
		"icon":     iconName,
		"ref":      ref,
	}
}

func taskFailed(task plugin.TableRow) bool {
	exit := str(task["exitstatus"])
	return exit != "" && exit != "OK"
}

func taskSummary(task plugin.TableRow) string {
	name := str(task["name"])
	if name == "" {
		name = str(task["type"])
	}
	id := str(task["id"])
	if id == "" {
		return name
	}
	return name + " " + id
}

func taskResourceName(task plugin.TableRow) string {
	node := str(task["node"])
	if ref, ok := task["ref"].(plugin.ResourceIdentity); ok && ref.Namespace != "" {
		node = ref.Namespace
	}
	name := taskSummary(task)
	if node == "" {
		return name
	}
	return node + "/" + name
}

func withinHours(v any, now time.Time, hours int) bool {
	sec := numInt(v)
	if sec <= 0 {
		return false
	}
	return now.Sub(time.Unix(sec, 0)) <= time.Duration(hours)*time.Hour
}

func optionalList(ctx context.Context, s *Session, path string) ([]plugin.TableRow, error) {
	rows, err := s.list(ctx, path)
	if err == nil {
		return rows, nil
	}
	if errors.Is(err, plugin.ErrNotFound) || errors.Is(err, plugin.ErrForbidden) || errors.Is(err, plugin.ErrNotSupported) {
		return []plugin.TableRow{}, nil
	}
	return nil, err
}

func sortTimeline(rows []plugin.TableRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return str(rows[i]["time"]) > str(rows[j]["time"])
	})
}
