package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

// RealtimeObserver exposes the minimal state required to evaluate realtime health.
type RealtimeObserver interface {
	ActiveConnections() int64
}

// Realtime evaluates realtime hub health, surfacing failure counters captured by monitoring instrumentation.
func Realtime(observer RealtimeObserver) monitoring.Check {
	return monitoring.NewCheck("realtime", func(ctx context.Context) monitoring.ProbeResult {
		start := time.Now()
		if observer == nil {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusDegraded,
				Details:  "realtime hub unavailable",
				Duration: time.Since(start),
			}
		}

		snapshot := monitoring.Snapshot()
		status := monitoring.StatusUp
		var details []string

		if snapshot.Realtime.Failures > 0 {
			status = monitoring.StatusDegraded
			details = append(details, fmt.Sprintf("%d failures", snapshot.Realtime.Failures))
		}
		if snapshot.Realtime.ActiveConnections < 0 {
			status = monitoring.StatusDegraded
			details = append(details, "negative connection count")
		}

		return monitoring.ProbeResult{
			Status:   status,
			Details:  strings.Join(details, "; "),
			Duration: time.Since(start),
		}
	})
}
