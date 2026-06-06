package kubernetes

import (
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func ResourceOverview(rc *plugin.RequestContext) (any, error) {
	s, k, name, err := resourceTarget(rc)
	if err != nil {
		return nil, err
	}
	o, err := s.get(rc, k, name)
	if err != nil {
		return nil, apiErr(err)
	}
	out := overviewRecord(k, o.Object)
	if k.redact {
		out["redacted"] = true
		delete(out, "data")
		delete(out, "stringData")
	}
	return out, nil
}

func overviewRecord(k kind, o obj) Row {
	row := commonRow(o)
	if k.extra != nil {
		for key, val := range k.extra(o) {
			row[key] = val
		}
	}
	out := Row{
		"apiVersion":      str(o, "apiVersion"),
		"kind":            str(o, "kind"),
		"name":            row["name"],
		"namespace":       row["namespace"],
		"uid":             row["uid"],
		"createdAt":       row["createdAt"],
		"resourceVersion": str(o, "metadata", "resourceVersion"),
		"labels":          mapField(o, "metadata", "labels"),
		"annotations":     mapField(o, "metadata", "annotations"),
		"ownerReferences": slice(o, "metadata", "ownerReferences"),
		"finalizers":      slice(o, "metadata", "finalizers"),
		"conditions":      slice(o, "status", "conditions"),
	}
	for key, val := range row {
		if key != "ref" {
			out[key] = val
		}
	}
	for key, val := range kindOverview(k.name, o) {
		out[key] = val
	}
	return out
}

func kindOverview(kindName string, o obj) Row {
	switch kindName {
	case "pod":
		return podOverview(o)
	case "deployment":
		return Row{
			"replicas":            i64(o, "spec", "replicas"),
			"strategy":            str(o, "spec", "strategy", "type"),
			"selector":            labelSelector(mapField(o, "spec", "selector", "matchLabels")),
			"generation":          i64(o, "metadata", "generation"),
			"observedGeneration":  i64(o, "status", "observedGeneration"),
			"unavailableReplicas": i64(o, "status", "unavailableReplicas"),
		}
	case "statefulset", "replicaset", "replicationcontroller":
		return Row{
			"replicas":           i64(o, "spec", "replicas"),
			"selector":           labelSelector(mapField(o, "spec", "selector", "matchLabels")),
			"generation":         i64(o, "metadata", "generation"),
			"observedGeneration": i64(o, "status", "observedGeneration"),
		}
	case "daemonset":
		return Row{
			"selector":           labelSelector(mapField(o, "spec", "selector", "matchLabels")),
			"generation":         i64(o, "metadata", "generation"),
			"observedGeneration": i64(o, "status", "observedGeneration"),
		}
	case "job":
		return Row{
			"parallelism":    i64(o, "spec", "parallelism"),
			"backoffLimit":   i64(o, "spec", "backoffLimit"),
			"startTime":      str(o, "status", "startTime"),
			"completionTime": str(o, "status", "completionTime"),
		}
	case "cronjob":
		return Row{
			"concurrencyPolicy": str(o, "spec", "concurrencyPolicy"),
			"lastSchedule":      str(o, "status", "lastScheduleTime"),
			"lastSuccessful":    str(o, "status", "lastSuccessfulTime"),
		}
	case "node":
		return Row{
			"os":             str(o, "status", "nodeInfo", "operatingSystem"),
			"architecture":   str(o, "status", "nodeInfo", "architecture"),
			"kernel":         str(o, "status", "nodeInfo", "kernelVersion"),
			"runtime":        str(o, "status", "nodeInfo", "containerRuntimeVersion"),
			"kubelet":        str(o, "status", "nodeInfo", "kubeletVersion"),
			"cpuAllocatable": str(o, "status", "allocatable", "cpu"),
			"memAllocatable": quantityBytes(o, "status", "allocatable", "memory"),
			"podAllocatable": str(o, "status", "allocatable", "pods"),
			"taints":         slice(o, "spec", "taints"),
			"addresses":      slice(o, "status", "addresses"),
		}
	case "service":
		return Row{
			"selector":        labelSelector(mapField(o, "spec", "selector")),
			"sessionAffinity": str(o, "spec", "sessionAffinity"),
		}
	case "ingress":
		return Row{"rules": slice(o, "spec", "rules"), "tls": slice(o, "spec", "tls")}
	case "persistentvolumeclaim":
		return Row{"requested": quantityBytes(o, "spec", "resources", "requests", "storage")}
	case "persistentvolume":
		return Row{"capacityBytes": quantityBytes(o, "spec", "capacity", "storage")}
	default:
		return nil
	}
}

func podOverview(o obj) Row {
	reqCPU, reqMem, limCPU, limMem := podResourceTotals(o)
	return Row{
		"phase":               str(o, "status", "phase"),
		"reason":              str(o, "status", "reason"),
		"message":             str(o, "status", "message"),
		"hostIP":              str(o, "status", "hostIP"),
		"qosClass":            str(o, "status", "qosClass"),
		"startTime":           str(o, "status", "startTime"),
		"serviceAccount":      str(o, "spec", "serviceAccountName"),
		"restartPolicy":       str(o, "spec", "restartPolicy"),
		"scheduler":           str(o, "spec", "schedulerName"),
		"priorityClass":       str(o, "spec", "priorityClassName"),
		"priority":            i64(o, "spec", "priority"),
		"containers":          int64(len(slice(o, "spec", "containers"))),
		"initContainers":      int64(len(slice(o, "spec", "initContainers"))),
		"volumes":             int64(len(slice(o, "spec", "volumes"))),
		"cpuRequest":          milliToCores(reqCPU),
		"cpuLimit":            milliToCores(limCPU),
		"memRequest":          reqMem,
		"memLimit":            limMem,
		"containerStatuses":   slice(o, "status", "containerStatuses"),
		"initContainerStatus": slice(o, "status", "initContainerStatuses"),
	}
}

func podResourceTotals(o obj) (cpuRequest, memRequest, cpuLimit, memLimit int64) {
	for _, c := range append(slice(o, "spec", "initContainers"), slice(o, "spec", "containers")...) {
		cm, ok := c.(obj)
		if !ok {
			continue
		}
		cpuRequest += quantityMilli(cm, "resources", "requests", "cpu")
		memRequest += quantityBytes(cm, "resources", "requests", "memory")
		cpuLimit += quantityMilli(cm, "resources", "limits", "cpu")
		memLimit += quantityBytes(cm, "resources", "limits", "memory")
	}
	return cpuRequest, memRequest, cpuLimit, memLimit
}

func quantityMilli(o obj, fields ...string) int64 {
	q, ok := quantity(o, fields...)
	if !ok {
		return 0
	}
	return q.MilliValue()
}

func quantityBytes(o obj, fields ...string) int64 {
	q, ok := quantity(o, fields...)
	if !ok {
		return 0
	}
	return q.Value()
}

func quantity(o obj, fields ...string) (resource.Quantity, bool) {
	raw := str(o, fields...)
	if raw == "" {
		return resource.Quantity{}, false
	}
	q, err := resource.ParseQuantity(raw)
	return q, err == nil
}

func overviewDetailConfig(k kind) plugin.ObjectDetailConfig {
	if k.name == "pod" {
		return podOverviewDetailConfig()
	}
	sections := []plugin.ObjectDetailSection{
		{Title: "Identity", Fields: []plugin.ObjectDetailField{
			{Key: "name", Label: "Name", Copy: true},
			{Key: "namespace", Label: "Namespace"},
			{Key: "kind", Label: "Kind"},
			{Key: "apiVersion", Label: "API version"},
			{Key: "uid", Label: "UID", Copy: true},
			{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime},
		}},
	}
	if fields := overviewFieldsForKind(k.name); len(fields) > 0 {
		sections = append(sections, plugin.ObjectDetailSection{Title: "Status", Fields: fields})
	}
	sections = append(sections, metadataOverviewSection())
	return plugin.ObjectDetailConfig{Sections: sections, RawToggle: true}
}

func genericOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{Sections: []plugin.ObjectDetailSection{
		{Title: "Identity", Fields: []plugin.ObjectDetailField{
			{Key: "name", Label: "Name", Copy: true},
			{Key: "namespace", Label: "Namespace"},
			{Key: "kind", Label: "Kind"},
			{Key: "apiVersion", Label: "API version"},
			{Key: "uid", Label: "UID", Copy: true},
			{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime},
		}},
		metadataOverviewSection(),
	}, RawToggle: true}
}

func podOverviewDetailConfig() plugin.ObjectDetailConfig {
	return plugin.ObjectDetailConfig{Sections: []plugin.ObjectDetailSection{
		{Title: "Summary", Fields: []plugin.ObjectDetailField{
			{Key: "name", Label: "Name", Copy: true},
			{Key: "namespace", Label: "Namespace"},
			{Key: "status", Label: "Status", Type: plugin.ColumnBadge, Severities: podSeverities},
			{Key: "ready", Label: "Ready"},
			{Key: "restarts", Label: "Restarts", Type: plugin.ColumnNumber},
			{Key: "qosClass", Label: "QoS class", Type: plugin.ColumnBadge},
			{Key: "createdAt", Label: "Created", Type: plugin.ColumnDateTime},
			{Key: "startTime", Label: "Started", Type: plugin.ColumnDateTime},
		}},
		{Title: "Placement", Fields: []plugin.ObjectDetailField{
			{Key: "node", Label: "Node", Copy: true},
			{Key: "podIP", Label: "Pod IP", Copy: true},
			{Key: "hostIP", Label: "Host IP", Copy: true},
			{Key: "serviceAccount", Label: "Service account"},
			{Key: "scheduler", Label: "Scheduler"},
			{Key: "priorityClass", Label: "Priority class"},
		}},
		{Title: "Resources", Fields: []plugin.ObjectDetailField{
			{Key: "cpuRequest", Label: "CPU request"},
			{Key: "cpuLimit", Label: "CPU limit"},
			{Key: "memRequest", Label: "Memory request", Type: plugin.ColumnBytes},
			{Key: "memLimit", Label: "Memory limit", Type: plugin.ColumnBytes},
			{Key: "containers", Label: "Containers", Type: plugin.ColumnNumber},
			{Key: "initContainers", Label: "Init containers", Type: plugin.ColumnNumber},
			{Key: "volumes", Label: "Volumes", Type: plugin.ColumnNumber},
		}},
		{Title: "Diagnostics", Fields: []plugin.ObjectDetailField{
			{Key: "reason", Label: "Reason"},
			{Key: "message", Label: "Message"},
			{Key: "conditions", Label: "Conditions", Type: plugin.ColumnJSON},
			{Key: "containerStatuses", Label: "Container statuses", Type: plugin.ColumnJSON},
		}},
		metadataOverviewSection(),
	}, RawToggle: true}
}

func overviewFieldsForKind(kindName string) []plugin.ObjectDetailField {
	fields := []plugin.ObjectDetailField{}
	add := func(key, label string, typ plugin.ColumnType) {
		fields = append(fields, plugin.ObjectDetailField{Key: key, Label: label, Type: typ})
	}
	switch kindName {
	case "node":
		add("status", "Status", plugin.ColumnBadge)
		add("roles", "Roles", "")
		add("cpuAllocatable", "CPU allocatable", "")
		add("memAllocatable", "Memory allocatable", plugin.ColumnBytes)
		add("podAllocatable", "Pod capacity", "")
		add("runtime", "Runtime", "")
		add("kubelet", "Kubelet", "")
	case "deployment":
		add("ready", "Ready", "")
		add("replicas", "Replicas", plugin.ColumnNumber)
		add("upToDate", "Up-to-date", plugin.ColumnNumber)
		add("available", "Available", plugin.ColumnNumber)
		add("unavailableReplicas", "Unavailable", plugin.ColumnNumber)
		add("strategy", "Strategy", "")
		add("selector", "Selector", "")
	case "statefulset", "replicaset", "replicationcontroller":
		add("ready", "Ready", "")
		add("replicas", "Replicas", plugin.ColumnNumber)
		add("selector", "Selector", "")
	case "daemonset":
		add("desired", "Desired", plugin.ColumnNumber)
		add("current", "Current", plugin.ColumnNumber)
		add("ready", "Ready", plugin.ColumnNumber)
		add("upToDate", "Up-to-date", plugin.ColumnNumber)
		add("available", "Available", plugin.ColumnNumber)
		add("selector", "Selector", "")
	case "job":
		add("completions", "Completions", "")
		add("active", "Active", plugin.ColumnNumber)
		add("duration", "Duration", "")
		add("parallelism", "Parallelism", plugin.ColumnNumber)
		add("backoffLimit", "Backoff limit", plugin.ColumnNumber)
	case "cronjob":
		add("schedule", "Schedule", "")
		add("timezone", "Timezone", "")
		add("suspend", "Suspended", plugin.ColumnBool)
		add("active", "Active jobs", plugin.ColumnNumber)
		add("lastSchedule", "Last schedule", plugin.ColumnDateTime)
		add("lastSuccessful", "Last success", plugin.ColumnDateTime)
	default:
		for _, col := range firstMeaningfulColumns(kindName) {
			fields = append(fields, plugin.ObjectDetailField{Key: col.Key, Label: col.Label, Type: col.Type, Severities: col.Severities})
		}
	}
	return fields
}

func firstMeaningfulColumns(kindName string) []plugin.Column {
	k, ok := kindByName(kindName)
	if !ok {
		return nil
	}
	out := []plugin.Column{}
	for _, c := range k.columns {
		if c.Key == "name" || c.Key == "namespace" || c.Key == "age" {
			continue
		}
		out = append(out, c)
		if len(out) == 8 {
			break
		}
	}
	return out
}

func metadataOverviewSection() plugin.ObjectDetailSection {
	return plugin.ObjectDetailSection{Title: "Metadata", Fields: []plugin.ObjectDetailField{
		{Key: "labels", Label: "Labels", Type: plugin.ColumnJSON},
		{Key: "annotations", Label: "Annotations", Type: plugin.ColumnJSON},
		{Key: "ownerReferences", Label: "Owners", Type: plugin.ColumnJSON},
		{Key: "finalizers", Label: "Finalizers", Type: plugin.ColumnJSON},
	}}
}
