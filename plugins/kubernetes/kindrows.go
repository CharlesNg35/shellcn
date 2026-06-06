package kubernetes

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Each mapper returns the cells specific to a kind; commonRow supplies the rest.

func podRow(o obj) Row {
	var total, ready, restarts int64
	for _, cs := range slice(o, "status", "containerStatuses") {
		c, ok := cs.(obj)
		if !ok {
			continue
		}
		total++
		if boolField(c, "ready") {
			ready++
		}
		restarts += i64(c, "restartCount")
	}
	status := str(o, "status", "phase")
	if reason := str(o, "status", "reason"); reason != "" {
		status = reason
	}
	return Row{
		"ready":    fmt.Sprintf("%d/%d", ready, total),
		"status":   status,
		"restarts": restarts,
		"node":     str(o, "spec", "nodeName"),
		"podIP":    str(o, "status", "podIP"),
		"ports":    podPorts(o),
	}
}

func podPorts(o obj) string {
	var ports []string
	for _, c := range slice(o, "spec", "containers") {
		cm, ok := c.(obj)
		if !ok {
			continue
		}
		for _, p := range slice(cm, "ports") {
			if pm, ok := p.(obj); ok {
				ports = append(ports, fmt.Sprintf("%d", i64(pm, "containerPort")))
			}
		}
	}
	return joinStrings(ports)
}

func deploymentRow(o obj) Row {
	return Row{
		"ready":     fmt.Sprintf("%d/%d", i64(o, "status", "readyReplicas"), i64(o, "spec", "replicas")),
		"upToDate":  i64(o, "status", "updatedReplicas"),
		"available": i64(o, "status", "availableReplicas"),
	}
}

func statefulSetRow(o obj) Row {
	return Row{"ready": fmt.Sprintf("%d/%d", i64(o, "status", "readyReplicas"), i64(o, "spec", "replicas"))}
}

func daemonSetRow(o obj) Row {
	return Row{
		"desired":   i64(o, "status", "desiredNumberScheduled"),
		"current":   i64(o, "status", "currentNumberScheduled"),
		"ready":     i64(o, "status", "numberReady"),
		"upToDate":  i64(o, "status", "updatedNumberScheduled"),
		"available": i64(o, "status", "numberAvailable"),
	}
}

func replicaSetRow(o obj) Row {
	return Row{
		"desired": i64(o, "spec", "replicas"),
		"current": i64(o, "status", "replicas"),
		"ready":   i64(o, "status", "readyReplicas"),
	}
}

func jobRow(o obj) Row {
	return Row{
		"completions": fmt.Sprintf("%d/%d", i64(o, "status", "succeeded"), i64(o, "spec", "completions")),
		"duration":    jobDuration(o),
		"active":      i64(o, "status", "active"),
	}
}

func cronJobRow(o obj) Row {
	return Row{
		"schedule":     str(o, "spec", "schedule"),
		"timezone":     str(o, "spec", "timeZone"),
		"suspend":      boolField(o, "spec", "suspend"),
		"active":       int64(len(slice(o, "status", "active"))),
		"lastSchedule": str(o, "status", "lastScheduleTime"),
	}
}

func serviceRow(o obj) Row {
	var ports []string
	for _, p := range slice(o, "spec", "ports") {
		if pm, ok := p.(obj); ok {
			ports = append(ports, fmt.Sprintf("%d/%s", i64(pm, "port"), str(pm, "protocol")))
		}
	}
	return Row{
		"type":       str(o, "spec", "type"),
		"clusterIP":  str(o, "spec", "clusterIP"),
		"externalIP": serviceExternalIP(o),
		"ports":      joinStrings(ports),
	}
}

func ingressRow(o obj) Row {
	var hosts []string
	for _, r := range slice(o, "spec", "rules") {
		if rm, ok := r.(obj); ok {
			if h := str(rm, "host"); h != "" {
				hosts = append(hosts, h)
			}
		}
	}
	return Row{
		"class":   str(o, "spec", "ingressClassName"),
		"hosts":   joinStrings(hosts),
		"address": ingressAddress(o),
		"ports":   ingressPorts(o),
	}
}

func pvcRow(o obj) Row {
	return Row{
		"status":       str(o, "status", "phase"),
		"volume":       str(o, "spec", "volumeName"),
		"capacity":     str(o, "status", "capacity", "storage"),
		"accessModes":  stringSlice(o, "spec", "accessModes"),
		"storageClass": str(o, "spec", "storageClassName"),
	}
}

func pvRow(o obj) Row {
	return Row{
		"capacity":     str(o, "spec", "capacity", "storage"),
		"accessModes":  stringSlice(o, "spec", "accessModes"),
		"status":       str(o, "status", "phase"),
		"claim":        str(o, "spec", "claimRef", "name"),
		"storageClass": str(o, "spec", "storageClassName"),
		"reclaim":      str(o, "spec", "persistentVolumeReclaimPolicy"),
		"reason":       str(o, "status", "reason"),
	}
}

func storageClassRow(o obj) Row {
	return Row{
		"default":     storageClassDefault(o),
		"provisioner": str(o, "provisioner"),
		"reclaim":     str(o, "reclaimPolicy"),
		"bindingMode": str(o, "volumeBindingMode"),
		"allowExpand": boolField(o, "allowVolumeExpansion"),
	}
}

func configMapRow(o obj) Row {
	return Row{"keys": int64(len(mapField(o, "data")))}
}

// secretRow never exposes secret values — only metadata.
func secretRow(o obj) Row {
	return Row{
		"type": str(o, "type"),
		"keys": int64(len(mapField(o, "data"))),
	}
}

func serviceAccountRow(o obj) Row {
	return Row{"secrets": int64(len(slice(o, "secrets")))}
}

func roleBindingRow(o obj) Row {
	var subs []string
	for _, sub := range slice(o, "subjects") {
		if sm, ok := sub.(obj); ok {
			subs = append(subs, str(sm, "kind")+"/"+str(sm, "name"))
		}
	}
	return Row{
		"role":     str(o, "roleRef", "kind") + "/" + str(o, "roleRef", "name"),
		"subjects": joinStrings(subs),
	}
}

func namespaceExtra(o obj) Row {
	return Row{"status": str(o, "status", "phase")}
}

func nodeRow(o obj) Row {
	status := "NotReady"
	for _, c := range slice(o, "status", "conditions") {
		if cm, ok := c.(obj); ok && str(cm, "type") == "Ready" && str(cm, "status") == "True" {
			status = "Ready"
		}
	}
	unschedulable := boolField(o, "spec", "unschedulable")
	if unschedulable {
		status += ",SchedulingDisabled"
	}
	var roles []string
	for label := range mapField(o, "metadata", "labels") {
		if r, ok := cutNodeRole(label); ok {
			roles = append(roles, r)
		}
	}
	return Row{
		"status":        status,
		"roles":         joinStrings(roles),
		"version":       str(o, "status", "nodeInfo", "kubeletVersion"),
		"unschedulable": unschedulable,
	}
}

func cutNodeRole(label string) (string, bool) {
	const prefix = "node-role.kubernetes.io/"
	if len(label) > len(prefix) && label[:len(prefix)] == prefix {
		return label[len(prefix):], true
	}
	return "", false
}

func eventRow(o obj) Row {
	involved := str(o, "involvedObject", "kind") + "/" + str(o, "involvedObject", "name")
	return Row{
		"type":    str(o, "type"),
		"reason":  str(o, "reason"),
		"object":  involved,
		"message": str(o, "message"),
		"count":   i64(o, "count"),
	}
}

func replicationControllerRow(o obj) Row {
	return Row{
		"desired": i64(o, "spec", "replicas"),
		"current": i64(o, "status", "replicas"),
		"ready":   i64(o, "status", "readyReplicas"),
	}
}

func pdbRow(o obj) Row {
	return Row{
		"minAvailable":       scalar(o, "spec", "minAvailable"),
		"maxUnavailable":     scalar(o, "spec", "maxUnavailable"),
		"allowedDisruptions": i64(o, "status", "disruptionsAllowed"),
		"currentHealthy":     i64(o, "status", "currentHealthy"),
		"desiredHealthy":     i64(o, "status", "desiredHealthy"),
	}
}

func priorityClassRow(o obj) Row {
	return Row{"value": i64(o, "value"), "globalDefault": boolField(o, "globalDefault")}
}

func runtimeClassRow(o obj) Row {
	return Row{"handler": str(o, "handler")}
}

func leaseRow(o obj) Row {
	return Row{"holder": str(o, "spec", "holderIdentity")}
}

func webhookConfigRow(o obj) Row {
	return Row{"webhooks": int64(len(slice(o, "webhooks")))}
}

func ingressClassRow(o obj) Row {
	return Row{"controller": str(o, "spec", "controller"), "parameters": ingressClassParameters(o)}
}

func endpointSliceRow(o obj) Row {
	return Row{
		"addressType": str(o, "addressType"),
		"endpoints":   int64(len(slice(o, "endpoints"))),
		"ports":       int64(len(slice(o, "ports"))),
	}
}

func crdDefRow(o obj) Row {
	return Row{
		"group": str(o, "spec", "group"),
		"kind":  str(o, "spec", "names", "kind"),
		"scope": str(o, "spec", "scope"),
	}
}

func hpaRow(o obj) Row {
	return Row{
		"reference": str(o, "spec", "scaleTargetRef", "kind") + "/" + str(o, "spec", "scaleTargetRef", "name"),
		"minPods":   i64(o, "spec", "minReplicas"),
		"maxPods":   i64(o, "spec", "maxReplicas"),
		"replicas":  i64(o, "status", "currentReplicas"),
	}
}

func endpointsRow(o obj) Row {
	var endpoints []string
	for _, subset := range slice(o, "subsets") {
		sm, ok := subset.(obj)
		if !ok {
			continue
		}
		var ports []string
		for _, p := range slice(sm, "ports") {
			if pm, ok := p.(obj); ok {
				ports = append(ports, fmt.Sprintf("%d", i64(pm, "port")))
			}
		}
		for _, a := range slice(sm, "addresses") {
			am, ok := a.(obj)
			if !ok {
				continue
			}
			address := firstNonEmpty(str(am, "ip"), str(am, "hostname"), str(am, "targetRef", "name"))
			if address == "" {
				continue
			}
			if len(ports) == 0 {
				endpoints = append(endpoints, address)
				continue
			}
			for _, port := range ports {
				endpoints = append(endpoints, address+":"+port)
			}
		}
	}
	return Row{"endpoints": joinStrings(endpoints)}
}

func networkPolicyRow(o obj) Row {
	return Row{
		"podSelector": selectorString(mapField(o, "spec", "podSelector")),
		"policyTypes": networkPolicyTypes(o),
	}
}

func shortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func jobDuration(o obj) string {
	startRaw := str(o, "status", "startTime")
	if startRaw == "" {
		return ""
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return ""
	}
	end := time.Now()
	if completionRaw := str(o, "status", "completionTime"); completionRaw != "" {
		completion, err := time.Parse(time.RFC3339, completionRaw)
		if err != nil {
			return ""
		}
		end = completion
	}
	if end.Before(start) {
		return ""
	}
	return shortDuration(end.Sub(start))
}

func serviceExternalIP(o obj) string {
	var values []string
	for _, ip := range slice(o, "spec", "externalIPs") {
		if s := fmt.Sprint(ip); s != "" {
			values = append(values, s)
		}
	}
	for _, ingress := range slice(o, "status", "loadBalancer", "ingress") {
		im, ok := ingress.(obj)
		if !ok {
			continue
		}
		if v := firstNonEmpty(str(im, "ip"), str(im, "hostname")); v != "" {
			values = append(values, v)
		}
	}
	return joinStrings(values)
}

func ingressAddress(o obj) string {
	var values []string
	for _, ingress := range slice(o, "status", "loadBalancer", "ingress") {
		im, ok := ingress.(obj)
		if !ok {
			continue
		}
		if v := firstNonEmpty(str(im, "ip"), str(im, "hostname")); v != "" {
			values = append(values, v)
		}
	}
	return joinStrings(values)
}

func ingressPorts(o obj) string {
	if len(slice(o, "spec", "tls")) > 0 {
		return "80, 443"
	}
	return "80"
}

func ingressClassParameters(o obj) string {
	params := mapField(o, "spec", "parameters")
	if len(params) == 0 {
		return ""
	}
	name := mapString(params, "name")
	kind := mapString(params, "kind")
	namespace := mapString(params, "namespace")
	if kind == "" || name == "" {
		return ""
	}
	if namespace != "" {
		name = namespace + "/" + name
	}
	return kind + "/" + name
}

func storageClassDefault(o obj) bool {
	annotations := mapField(o, "metadata", "annotations")
	for _, key := range []string{
		"storageclass.kubernetes.io/is-default-class",
		"storageclass.beta.kubernetes.io/is-default-class",
	} {
		if strings.EqualFold(fmt.Sprint(annotations[key]), "true") {
			return true
		}
	}
	return false
}

func stringSlice(o obj, fields ...string) string {
	var out []string
	for _, item := range slice(o, fields...) {
		if s := fmt.Sprint(item); s != "" {
			out = append(out, s)
		}
	}
	return joinStrings(out)
}

func selectorString(selector map[string]any) string {
	if len(selector) == 0 {
		return ""
	}
	var parts []string
	labels := mapField(selector, "matchLabels")
	for _, key := range sortedKeys(labels) {
		parts = append(parts, key+"="+fmt.Sprint(labels[key]))
	}
	for _, raw := range slice(selector, "matchExpressions") {
		expr, ok := raw.(obj)
		if !ok {
			continue
		}
		key := str(expr, "key")
		op := str(expr, "operator")
		values := stringSlice(expr, "values")
		if values != "" {
			parts = append(parts, key+" "+op+" ("+values+")")
		} else if key != "" || op != "" {
			parts = append(parts, strings.TrimSpace(key+" "+op))
		}
	}
	return joinStrings(parts)
}

func networkPolicyTypes(o obj) string {
	types := stringSlice(o, "spec", "policyTypes")
	if types != "" {
		return types
	}
	var out []string
	if len(slice(o, "spec", "ingress")) > 0 {
		out = append(out, "Ingress")
	}
	if len(slice(o, "spec", "egress")) > 0 {
		out = append(out, "Egress")
	}
	if len(out) == 0 {
		out = append(out, "Ingress")
	}
	return joinStrings(out)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mapString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
