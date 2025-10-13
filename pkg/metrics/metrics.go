package metrics

import (
	"time"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

// Deprecated: use monitoring.RecordAuthAttempt directly.
func RecordAuthAttempt(result string) {
	monitoring.RecordAuthAttempt(result)
}

// Deprecated: use monitoring.RecordPermissionCheck directly.
func RecordPermissionCheck(permissionID, result string) {
	monitoring.RecordPermissionCheck(permissionID, result)
}

// Deprecated: use monitoring.ObserveAPILatency directly.
func ObserveAPILatency(method, path, status string, duration time.Duration) {
	monitoring.ObserveAPILatency(method, path, status, duration)
}

// Deprecated: use monitoring.AdjustActiveSessions directly.
func AdjustActiveSessions(delta int64) {
	monitoring.AdjustActiveSessions(delta)
}

// Deprecated: use monitoring.RecordSessionClosed directly.
func RecordSessionClosed(duration time.Duration) {
	monitoring.RecordSessionClosed(duration)
}

// Deprecated: use monitoring.RecordVaultOperation directly.
func RecordVaultOperation(operation, result string) {
	monitoring.RecordVaultOperation(operation, result)
}

// Deprecated: use monitoring.RecordVaultPayloadRequest directly.
func RecordVaultPayloadRequest(result string) {
	monitoring.RecordVaultPayloadRequest(result)
}
