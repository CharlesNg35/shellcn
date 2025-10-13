package checks

import (
	"context"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

const defaultMaintenanceMaxAge = 6 * time.Hour

// Maintenance verifies that background jobs run successfully within the expected interval.
// When maxAge is zero, a sensible default window (6h) is used.
func Maintenance(maxAge time.Duration) monitoring.Check {
	if maxAge <= 0 {
		maxAge = defaultMaintenanceMaxAge
	}

	return monitoring.NewCheck("maintenance", func(ctx context.Context) monitoring.ProbeResult {
		start := time.Now()
		summary := monitoring.Snapshot()
		now := time.Now()

		if len(summary.Maintenance.Jobs) == 0 {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusUp,
				Details:  "no maintenance jobs registered",
				Duration: time.Since(start),
			}
		}

		status := monitoring.StatusUp
		var failures []string

		for _, job := range summary.Maintenance.Jobs {
			if job.TotalRuns == 0 {
				failures = append(failures, job.Job+": pending first run")
				continue
			}

			if job.ConsecutiveFailures > 0 {
				status = worstStatus(status, monitoring.StatusDown)
				failures = append(failures, job.Job+": consecutive failures")
			}

			if maxAge > 0 && !job.LastRunAt.IsZero() && now.Sub(job.LastRunAt) > maxAge {
				status = worstStatus(status, monitoring.StatusDegraded)
				failures = append(failures, job.Job+": stale run "+job.LastRunAt.UTC().Format(time.RFC3339))
			}
		}

		details := strings.Join(failures, "; ")

		return monitoring.ProbeResult{
			Status:   status,
			Details:  details,
			Duration: time.Since(start),
		}
	})
}

func worstStatus(current, candidate monitoring.ProbeStatus) monitoring.ProbeStatus {
	if current == monitoring.StatusDown || candidate == monitoring.StatusDown {
		return monitoring.StatusDown
	}
	if current == monitoring.StatusDegraded || candidate == monitoring.StatusDegraded {
		return monitoring.StatusDegraded
	}
	return monitoring.StatusUp
}
