package monitoring

import (
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Options control monitoring module configuration.
type Options struct {
	// Namespace configures the Prometheus namespace. Defaults to "shellcn".
	Namespace string
	// DisableGoCollector skips registration of the Go runtime collector when true.
	DisableGoCollector bool
	// DisableProcessCollector skips registration of the process collector when true.
	DisableProcessCollector bool
}

// Module coordinates Prometheus metrics collectors, runtime health probes, and summary state.
type Module struct {
	registry *prometheus.Registry
	metrics  *collectors
	stats    *statStore
	health   *HealthManager
}

// NewModule constructs a monitoring module with its own Prometheus registry.
func NewModule(opts Options) (*Module, error) {
	namespace := opts.Namespace
	if namespace == "" {
		namespace = "shellcn"
	}

	registry := prometheus.NewRegistry()
	if !opts.DisableGoCollector {
		if err := registry.Register(prometheus.NewGoCollector()); err != nil {
			return nil, err
		}
	}
	if !opts.DisableProcessCollector {
		if err := registry.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{})); err != nil {
			return nil, err
		}
	}

	metrics := newCollectors(namespace)
	for _, collector := range metrics.all() {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}

	module := &Module{
		registry: registry,
		metrics:  metrics,
		stats:    newStatStore(),
		health:   NewHealthManager(),
	}
	return module, nil
}

// Registry exposes the underlying Prometheus registry.
func (m *Module) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

// Handler returns an http.Handler serving Prometheus metrics for this module.
func (m *Module) Handler() http.Handler {
	if m == nil || m.registry == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Metrics returns the module's collector set. For internal use.
func (m *Module) Metrics() *collectors {
	if m == nil {
		return nil
	}
	return m.metrics
}

// Stats returns the runtime statistics store backing the monitoring summary.
func (m *Module) Stats() *statStore {
	if m == nil {
		return nil
	}
	return m.stats
}

// Health exposes the health manager responsible for liveness and readiness probes.
func (m *Module) Health() *HealthManager {
	if m == nil {
		return nil
	}
	return m.health
}

var globalModule atomic.Pointer[Module]

// SetModule configures the process-wide monitoring module used by instrumentation helpers.
func SetModule(module *Module) {
	if module == nil {
		return
	}
	globalModule.Store(module)
}

// CurrentModule returns the process-wide monitoring module, or nil when unset.
func CurrentModule() *Module {
	return globalModule.Load()
}

// ensureModule returns the current module or nil when unset.
func ensureModule() *Module {
	return globalModule.Load()
}
