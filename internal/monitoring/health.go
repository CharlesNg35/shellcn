package monitoring

import (
	"context"
	"errors"
	"time"
)

// ProbeStatus encodes the outcome of a health probe.
type ProbeStatus string

const (
	StatusUp       ProbeStatus = "up"
	StatusDown     ProbeStatus = "down"
	StatusDegraded ProbeStatus = "degraded"
)

// ProbeResult captures a single dependency check outcome.
type ProbeResult struct {
	Component string        `json:"component"`
	Status    ProbeStatus   `json:"status"`
	Details   string        `json:"details,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// HealthReport aggregates probe results for a liveness or readiness evaluation.
type HealthReport struct {
	Success bool          `json:"success"`
	Status  ProbeStatus   `json:"status"`
	Checks  []ProbeResult `json:"checks"`
}

// Check encapsulates a single dependency probe.
type Check struct {
	Name string
	Run  func(ctx context.Context) ProbeResult
}

// NewCheck constructs a health check with the provided name and function.
func NewCheck(name string, fn func(ctx context.Context) ProbeResult) Check {
	if fn == nil {
		fn = func(context.Context) ProbeResult {
			return ProbeResult{
				Component: name,
				Status:    StatusDown,
				Details:   "probe not implemented",
			}
		}
	}
	return Check{Name: name, Run: fn}
}

// HealthManager coordinates liveness and readiness probes.
type HealthManager struct {
	livenessChecks  []Check
	readinessChecks []Check
}

// NewHealthManager constructs an empty health manager.
func NewHealthManager() *HealthManager {
	return &HealthManager{}
}

// RegisterLiveness appends a liveness probe.
func (m *HealthManager) RegisterLiveness(check Check) {
	if check.Name == "" {
		return
	}
	m.livenessChecks = append(m.livenessChecks, check)
}

// RegisterReadiness appends a readiness probe.
func (m *HealthManager) RegisterReadiness(check Check) {
	if check.Name == "" {
		return
	}
	m.readinessChecks = append(m.readinessChecks, check)
}

// EvaluateLiveness executes all configured liveness checks.
func (m *HealthManager) EvaluateLiveness(ctx context.Context) HealthReport {
	return m.evaluate(ctx, m.livenessChecks)
}

// EvaluateReadiness executes all configured readiness checks.
func (m *HealthManager) EvaluateReadiness(ctx context.Context) HealthReport {
	return m.evaluate(ctx, m.readinessChecks)
}

func (m *HealthManager) evaluate(ctx context.Context, checks []Check) HealthReport {
	report := HealthReport{
		Success: true,
		Status:  StatusUp,
		Checks:  make([]ProbeResult, 0, len(checks)),
	}

	if len(checks) == 0 {
		return report
	}

	for _, check := range checks {
		result := runCheck(ctx, check)
		report.Checks = append(report.Checks, result)

		switch result.Status {
		case StatusDown:
			report.Success = false
			report.Status = StatusDown
		case StatusDegraded:
			if report.Status != StatusDown {
				report.Success = false
				report.Status = StatusDegraded
			}
		}
	}
	return report
}

func runCheck(ctx context.Context, check Check) ProbeResult {
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	var (
		result   ProbeResult
		panicked bool
	)

	defer func() {
		if rec := recover(); rec != nil {
			panicked = true
			details := "panic recovered"
			switch v := rec.(type) {
			case string:
				details = v
			case error:
				details = v.Error()
			}
			result = ProbeResult{
				Status:   StatusDown,
				Details:  details,
				Duration: time.Since(start),
			}
		}
	}()

	result = check.Run(ctx)

	if result.Status == "" {
		result.Status = StatusDown
	}
	if !panicked && result.Duration == 0 {
		result.Duration = time.Since(start)
	}
	result.Component = check.Name
	return result
}

// MergeReports combines liveness and readiness results to a single payload.
func MergeReports(live, ready HealthReport) HealthReport {
	checks := append([]ProbeResult(nil), live.Checks...)
	checks = append(checks, ready.Checks...)

	status := StatusUp
	success := true

	for _, r := range checks {
		switch r.Status {
		case StatusDown:
			status = StatusDown
			success = false
		case StatusDegraded:
			if status != StatusDown {
				status = StatusDegraded
				success = false
			}
		}
	}

	return HealthReport{
		Success: success,
		Status:  status,
		Checks:  checks,
	}
}

// ResultFromError converts an error into a ProbeResult with sensible defaults.
func ResultFromError(component string, err error, duration time.Duration) ProbeResult {
	if duration < 0 {
		duration = 0
	}
	if err == nil {
		return ProbeResult{Component: component, Status: StatusUp, Duration: duration}
	}

	status := StatusDown
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		status = StatusDegraded
	}

	return ProbeResult{
		Component: component,
		Status:    status,
		Details:   err.Error(),
		Duration:  duration,
	}
}
