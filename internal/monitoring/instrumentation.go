package monitoring

import (
	"strings"
	"time"
)

// RecordAuthAttempt increments the auth attempt counter.
func RecordAuthAttempt(result string) {
	module := ensureModule()
	if module == nil {
		return
	}
	label := normalizeLabel(result)
	module.metrics.authAttempts.WithLabelValues(label).Inc()
	module.stats.recordAuth(label)
}

// RecordPermissionCheck records the outcome of a permission evaluation.
func RecordPermissionCheck(permissionID, result string) {
	module := ensureModule()
	if module == nil {
		return
	}
	perm := strings.TrimSpace(permissionID)
	if perm == "" {
		perm = "unknown"
	}
	label := normalizeLabel(result)
	module.metrics.permissionChecks.WithLabelValues(perm, label).Inc()
	module.stats.recordPermission(label)
}

// ObserveAPILatency captures the HTTP request latency for the supplied route.
func ObserveAPILatency(method, path, status string, duration time.Duration) {
	module := ensureModule()
	if module == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "UNKNOWN"
	}
	path = sanitizePath(path)
	if path == "" {
		path = "unknown"
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "unknown"
	}
	module.metrics.apiLatency.WithLabelValues(method, path, status).Observe(duration.Seconds())
}

// AdjustActiveSessions modifies the live session gauge by delta.
func AdjustActiveSessions(delta int64) {
	module := ensureModule()
	if module == nil {
		return
	}
	if delta == 0 {
		return
	}
	module.metrics.activeSessions.Add(float64(delta))
	module.stats.adjustActiveSessions(delta)
	if module.stats.activeSessions.Load() < 0 {
		module.stats.activeSessions.Store(0)
		module.metrics.activeSessions.Set(0)
	}
}

// RecordVaultOperation increments the vault operation counter.
func RecordVaultOperation(operation, result string) {
	module := ensureModule()
	if module == nil {
		return
	}
	op := normalizeLabel(operation)
	res := normalizeLabel(result)
	module.metrics.vaultOperations.WithLabelValues(op, res).Inc()
}

// RecordVaultPayloadRequest increments the payload request counter by result.
func RecordVaultPayloadRequest(result string) {
	module := ensureModule()
	if module == nil {
		return
	}
	module.metrics.vaultPayloadRequests.WithLabelValues(normalizeLabel(result)).Inc()
}

// RecordSessionClosed records an observed session duration.
func RecordSessionClosed(duration time.Duration) {
	module := ensureModule()
	if module == nil {
		return
	}
	if duration < 0 {
		duration = 0
	}
	observeDuration(module.metrics.sessionDuration, duration)
	module.stats.recordSessionDuration(duration)
}

// RecordRealtimeConnection adjusts the websocket connection gauge.
func RecordRealtimeConnection(delta int64) {
	module := ensureModule()
	if module == nil {
		return
	}
	if delta == 0 {
		return
	}
	module.metrics.realtimeConnections.Add(float64(delta))
	module.stats.recordRealtimeConnection(delta)
	if module.stats.realtimeConnections.Load() < 0 {
		module.stats.realtimeConnections.Store(0)
		module.metrics.realtimeConnections.Set(0)
	}
}

// RecordRealtimeSubscription tracks subscribe/unsubscribe events.
func RecordRealtimeSubscription(stream, action string) {
	module := ensureModule()
	if module == nil {
		return
	}
	stream = normalizePath(stream)
	if stream == "" {
		stream = "unknown"
	}
	action = normalizeLabel(action)
	module.metrics.realtimeSubscriptions.WithLabelValues(stream, action).Inc()
}

// RecordRealtimeBroadcast increments broadcast counters per stream.
func RecordRealtimeBroadcast(stream string) {
	module := ensureModule()
	if module == nil {
		return
	}
	stream = normalizePath(stream)
	if stream == "" {
		stream = "unknown"
	}
	module.metrics.realtimeBroadcasts.WithLabelValues(stream).Inc()
	module.stats.recordRealtimeBroadcast(stream)
}

// RecordRealtimeFailure snapshots a realtime failure occurrence.
func RecordRealtimeFailure(stream, failureType, message string) {
	module := ensureModule()
	if module == nil {
		return
	}
	stream = normalizePath(stream)
	if stream == "" {
		stream = "unknown"
	}
	failureType = normalizeLabel(failureType)
	if failureType == "" {
		failureType = "unknown"
	}
	module.metrics.realtimeFailures.WithLabelValues(stream, failureType).Inc()
	module.stats.recordRealtimeFailure(FailureRecord{
		Stream:   stream,
		Type:     failureType,
		Message:  strings.TrimSpace(message),
		Occurred: time.Now(),
	})
}

// RecordMaintenanceRun records the completion of a maintenance job.
func RecordMaintenanceRun(job, result, message string, duration time.Duration) {
	module := ensureModule()
	if module == nil {
		return
	}
	jobID := normalizeLabel(job)
	if jobID == "" {
		jobID = "unknown"
	}
	result = normalizeLabel(result)
	if result == "" {
		result = "unknown"
	}
	module.metrics.maintenanceRuns.WithLabelValues(jobID, result).Inc()
	observeDuration(module.metrics.maintenanceDuration.WithLabelValues(jobID), duration)
	if result == "success" {
		module.metrics.maintenanceLastRun.WithLabelValues(jobID).Set(float64(time.Now().Unix()))
	}
	stats := module.stats.maintenanceEntry(jobID)
	stats.record(result, strings.TrimSpace(message), duration)
}

// RecordProtocolLaunch captures protocol launch metrics and runtime stats.
func RecordProtocolLaunch(protocolID, result, message string, duration time.Duration) {
	module := ensureModule()
	if module == nil {
		return
	}
	id := normalizeLabel(protocolID)
	if id == "" {
		id = "unknown"
	}
	result = normalizeLabel(result)
	if result == "" {
		result = "unknown"
	}
	module.metrics.protocolLaunches.WithLabelValues(id, result).Inc()
	observeDuration(module.metrics.protocolLaunchLatency.WithLabelValues(id), duration)
	stats := module.stats.protocolEntry(id)
	stats.record(result, strings.TrimSpace(message), duration)
}

// RecordSessionShareEvent tracks participant and sharing lifecycle events.
func RecordSessionShareEvent(event string) {
	module := ensureModule()
	if module == nil {
		return
	}
	label := normalizeLabel(event)
	module.metrics.sessionShareEvents.WithLabelValues(label).Inc()
}

// RecordSessionRecordingEvent tracks recording lifecycle activity per session.
func RecordSessionRecordingEvent(event string) {
	module := ensureModule()
	if module == nil {
		return
	}
	label := normalizeLabel(event)
	module.metrics.sessionRecordingEvents.WithLabelValues(label).Inc()
}

func normalizeLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "unknown"
	}
	return value
}

func sanitizePath(path string) string {
	if path == "" {
		return ""
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "/" {
		return "root"
	}
	return normalizePath(path)
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")
	path = strings.ReplaceAll(path, " ", "_")
	if path == "" {
		return "root"
	}
	return path
}
