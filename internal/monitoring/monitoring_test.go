package monitoring_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/monitoring/checks"
)

func setupModule(t *testing.T) *monitoring.Module {
	t.Helper()

	mod, err := monitoring.NewModule(monitoring.Options{})
	require.NoError(t, err)
	monitoring.SetModule(mod)
	return mod
}

func TestSummaryAggregatesMetrics(t *testing.T) {
	t.Parallel()
	setupModule(t)

	monitoring.RecordAuthAttempt("success")
	monitoring.RecordAuthAttempt("failure")
	monitoring.RecordPermissionCheck("connection.view", "allowed")
	monitoring.RecordPermissionCheck("connection.view", "denied")
	monitoring.AdjustActiveSessions(1)
	monitoring.RecordSessionClosed(2 * time.Second)
	monitoring.RecordRealtimeConnection(1)
	monitoring.RecordRealtimeBroadcast("notifications")
	monitoring.RecordRealtimeFailure("notifications", "backpressure", "drop")
	monitoring.RecordMaintenanceRun("session_cleanup", "success", "", time.Second)
	monitoring.RecordProtocolLaunch("ssh", "success", "", 500*time.Millisecond)

	summary := monitoring.Snapshot()
	require.Equal(t, uint64(2), summary.Auth.Success+summary.Auth.Failure)
	require.Equal(t, int64(1), summary.Sessions.Active)
	require.GreaterOrEqual(t, summary.Realtime.Failures, uint64(1))
	require.NotEmpty(t, summary.Maintenance.Jobs)
	require.NotEmpty(t, summary.Protocols)
}

func TestHealthManagerEvaluate(t *testing.T) {
	t.Parallel()

	manager := monitoring.NewHealthManager()
	manager.RegisterReadiness(monitoring.NewCheck("database", func(ctx context.Context) monitoring.ProbeResult {
		return monitoring.ProbeResult{Status: monitoring.StatusUp}
	}))
	manager.RegisterReadiness(monitoring.NewCheck("redis", func(ctx context.Context) monitoring.ProbeResult {
		return monitoring.ProbeResult{Status: monitoring.StatusDown, Details: "connection refused"}
	}))

	report := manager.EvaluateReadiness(context.Background())
	require.False(t, report.Success)
	require.Equal(t, monitoring.StatusDown, report.Status)
	require.Len(t, report.Checks, 2)
}

func TestMaintenanceCheck(t *testing.T) {
	t.Parallel()
	setupModule(t)

	monitoring.RecordMaintenanceRun("session_cleanup", "success", "", time.Second)
	monitoring.RecordMaintenanceRun("audit_cleanup", "failure", "timeout", time.Second)

	check := checks.Maintenance(0)
	result := check.Run(context.Background())
	require.Equal(t, monitoring.StatusDown, result.Status)
	require.NotEmpty(t, result.Details)
}
