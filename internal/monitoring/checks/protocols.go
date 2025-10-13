package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

// DriverProbe encapsulates a protocol driver's health check.
type DriverProbe struct {
	ID    string
	Check func(context.Context) error
}

// ProtocolDrivers executes driver health checks and reports aggregated status.
func ProtocolDrivers(drivers []DriverProbe) monitoring.Check {
	return monitoring.NewCheck("protocol_drivers", func(ctx context.Context) monitoring.ProbeResult {
		start := time.Now()
		if len(drivers) == 0 {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusDegraded,
				Details:  "no driver probes registered",
				Duration: time.Since(start),
			}
		}

		status := monitoring.StatusUp
		var failures []string

		for _, probe := range drivers {
			if probe.Check == nil {
				status = worstStatus(status, monitoring.StatusDegraded)
				failures = append(failures, fmt.Sprintf("%s: check unavailable", probe.ID))
				continue
			}
			if err := probe.Check(ctx); err != nil {
				status = monitoring.StatusDown
				failures = append(failures, fmt.Sprintf("%s: %v", probe.ID, err))
			}
		}

		return monitoring.ProbeResult{
			Status:   status,
			Details:  strings.Join(failures, "; "),
			Duration: time.Since(start),
		}
	})
}
