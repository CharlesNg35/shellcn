package checks

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

const defaultDatabaseTimeout = 2 * time.Second

// Database returns a readiness probe that pings the configured database handle.
func Database(db *gorm.DB, timeout time.Duration) monitoring.Check {
	return monitoring.NewCheck("database", func(ctx context.Context) monitoring.ProbeResult {
		start := time.Now()
		if db == nil {
			return monitoring.ProbeResult{
				Status:   monitoring.StatusDown,
				Details:  "database not configured",
				Duration: time.Since(start),
			}
		}

		sqlDB, err := db.DB()
		if err != nil {
			return monitoring.ResultFromError("database", err, time.Since(start))
		}

		probeCtx, cancel := context.WithTimeout(ctx, chooseTimeout(timeout, defaultDatabaseTimeout))
		defer cancel()

		if err := sqlDB.PingContext(probeCtx); err != nil {
			return monitoring.ResultFromError("database", err, time.Since(start))
		}

		return monitoring.ProbeResult{
			Status:   monitoring.StatusUp,
			Duration: time.Since(start),
		}
	})
}

func chooseTimeout(provided, fallback time.Duration) time.Duration {
	if provided <= 0 {
		return fallback
	}
	return provided
}
