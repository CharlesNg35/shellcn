package checks

import (
	"context"
	"time"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

const defaultRedisTimeout = 2 * time.Second

// RedisPinger represents the minimal interface required to probe a redis connection.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// Redis returns a readiness probe for the configured Redis cache. When disabled, the probe
// reports StatusUp with a descriptive message to aid operators.
func Redis(client RedisPinger, enabled bool, timeout time.Duration) monitoring.Check {
	return monitoring.NewCheck("redis", func(ctx context.Context) monitoring.ProbeResult {
		start := time.Now()
		if !enabled {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusUp,
				Details:  "redis disabled",
				Duration: time.Since(start),
			}
		}
		if client == nil {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusDegraded,
				Details:  "redis unavailable",
				Duration: time.Since(start),
			}
		}

		probeCtx, cancel := context.WithTimeout(ctx, chooseTimeout(timeout, defaultRedisTimeout))
		defer cancel()

		if err := client.Ping(probeCtx); err != nil {
			return monitoring.ResultFromError("redis", err, time.Since(start))
		}

		return monitoring.ProbeResult{
			Status:   monitoring.StatusUp,
			Duration: time.Since(start),
		}
	})
}
