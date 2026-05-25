package telemetry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"sync"
	"time"
)

// Check reports whether a subsystem is healthy.
type Check func(ctx context.Context) error

// Health aggregates named liveness/readiness checks behind a status endpoint.
type Health struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewHealth returns an empty health registry.
func NewHealth() *Health {
	return &Health{checks: make(map[string]Check)}
}

// Register adds (or replaces) a named check.
func (h *Health) Register(name string, c Check) {
	h.mu.Lock()
	h.checks[name] = c
	h.mu.Unlock()
}

type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Handler runs every check and reports overall status. Any failing check yields
// HTTP 503 so a load balancer can route around an unhealthy instance.
func (h *Health) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		h.mu.RLock()
		checks := make(map[string]Check, len(h.checks))
		maps.Copy(checks, h.checks)
		h.mu.RUnlock()

		resp := healthResponse{Status: "ok", Checks: map[string]string{}}
		healthy := true
		for name, c := range checks {
			if err := c(ctx); err != nil {
				healthy = false
				resp.Checks[name] = err.Error()
			} else {
				resp.Checks[name] = "ok"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if !healthy {
			resp.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
