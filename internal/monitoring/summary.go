package monitoring

import "time"

// Summary surfaces aggregated monitoring data for administrative dashboards.
type Summary struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Auth        AuthSummary        `json:"auth"`
	Permissions PermissionSummary  `json:"permissions"`
	Sessions    SessionSummary     `json:"sessions"`
	Realtime    RealtimeSummary    `json:"realtime"`
	Maintenance MaintenanceSummary `json:"maintenance"`
	Protocols   []ProtocolSummary  `json:"protocols"`
}

type AuthSummary struct {
	Success uint64 `json:"success"`
	Failure uint64 `json:"failure"`
	Error   uint64 `json:"error"`
}

type PermissionSummary struct {
	Allowed uint64 `json:"allowed"`
	Denied  uint64 `json:"denied"`
	Error   uint64 `json:"error"`
}

type SessionSummary struct {
	Active                 int64         `json:"active"`
	Completed              uint64        `json:"completed"`
	AverageDurationSeconds float64       `json:"average_duration_seconds"`
	LastDuration           time.Duration `json:"last_duration"`
	LastEndedAt            time.Time     `json:"last_ended_at"`
}

type FailureRecord struct {
	Stream   string    `json:"stream"`
	Type     string    `json:"type"`
	Message  string    `json:"message"`
	Occurred time.Time `json:"occurred_at"`
}

type RealtimeSummary struct {
	ActiveConnections int64          `json:"active_connections"`
	Broadcasts        uint64         `json:"broadcasts"`
	Failures          uint64         `json:"failures"`
	LastFailure       *FailureRecord `json:"last_failure,omitempty"`
}

type MaintenanceSummary struct {
	Jobs []MaintenanceJobSummary `json:"jobs"`
}

type MaintenanceJobSummary struct {
	Job                 string        `json:"job"`
	LastStatus          string        `json:"last_status"`
	LastRunAt           time.Time     `json:"last_run_at"`
	LastDuration        time.Duration `json:"last_duration"`
	LastError           string        `json:"last_error,omitempty"`
	ConsecutiveFailures uint64        `json:"consecutive_failures"`
	ConsecutiveSuccess  uint64        `json:"consecutive_success"`
	LastSuccessAt       time.Time     `json:"last_success_at"`
	TotalRuns           uint64        `json:"total_runs"`
}

type ProtocolSummary struct {
	Protocol              string        `json:"protocol"`
	Success               uint64        `json:"success"`
	Failure               uint64        `json:"failure"`
	LastStatus            string        `json:"last_status"`
	LastDuration          time.Duration `json:"last_duration"`
	LastCompletedAt       time.Time     `json:"last_completed_at"`
	LastError             string        `json:"last_error,omitempty"`
	AverageLatencySeconds float64       `json:"average_latency_seconds"`
}

// Snapshot returns a point-in-time summary from the current module when configured.
func Snapshot() Summary {
	if module := ensureModule(); module != nil && module.stats != nil {
		return module.stats.summary()
	}
	return Summary{GeneratedAt: time.Now()}
}
