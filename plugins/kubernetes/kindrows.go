package kubernetes

import "fmt"

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
	}
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
		"active":      i64(o, "status", "active"),
	}
}

func cronJobRow(o obj) Row {
	return Row{
		"schedule":     str(o, "spec", "schedule"),
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
		"type":      str(o, "spec", "type"),
		"clusterIP": str(o, "spec", "clusterIP"),
		"ports":     joinStrings(ports),
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
		"class": str(o, "spec", "ingressClassName"),
		"hosts": joinStrings(hosts),
	}
}

func pvcRow(o obj) Row {
	return Row{
		"status":       str(o, "status", "phase"),
		"volume":       str(o, "spec", "volumeName"),
		"capacity":     str(o, "status", "capacity", "storage"),
		"storageClass": str(o, "spec", "storageClassName"),
	}
}

func pvRow(o obj) Row {
	return Row{
		"capacity":     str(o, "spec", "capacity", "storage"),
		"status":       str(o, "status", "phase"),
		"claim":        str(o, "spec", "claimRef", "name"),
		"storageClass": str(o, "spec", "storageClassName"),
		"reclaim":      str(o, "spec", "persistentVolumeReclaimPolicy"),
	}
}

func storageClassRow(o obj) Row {
	return Row{
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
	var roles []string
	for label := range mapField(o, "metadata", "labels") {
		if r, ok := cutNodeRole(label); ok {
			roles = append(roles, r)
		}
	}
	return Row{
		"status":  status,
		"roles":   joinStrings(roles),
		"version": str(o, "status", "nodeInfo", "kubeletVersion"),
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
		"minAvailable":   scalar(o, "spec", "minAvailable"),
		"maxUnavailable": scalar(o, "spec", "maxUnavailable"),
		"currentHealthy": i64(o, "status", "currentHealthy"),
		"desiredHealthy": i64(o, "status", "desiredHealthy"),
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
	return Row{"controller": str(o, "spec", "controller")}
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
