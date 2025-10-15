package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type collectors struct {
	authAttempts           *prometheus.CounterVec
	permissionChecks       *prometheus.CounterVec
	activeSessions         prometheus.Gauge
	apiLatency             *prometheus.HistogramVec
	vaultOperations        *prometheus.CounterVec
	vaultPayloadRequests   *prometheus.CounterVec
	realtimeConnections    prometheus.Gauge
	realtimeBroadcasts     *prometheus.CounterVec
	realtimeFailures       *prometheus.CounterVec
	realtimeSubscriptions  *prometheus.CounterVec
	maintenanceRuns        *prometheus.CounterVec
	maintenanceDuration    *prometheus.HistogramVec
	maintenanceLastRun     *prometheus.GaugeVec
	protocolLaunches       *prometheus.CounterVec
	protocolLaunchLatency  *prometheus.HistogramVec
	sessionDuration        prometheus.Histogram
	sessionShareEvents     *prometheus.CounterVec
	sessionRecordingEvents *prometheus.CounterVec
}

func newCollectors(namespace string) *collectors {
	buckets := prometheus.DefBuckets
	sessionBuckets := []float64{
		1, 5, 15, 30, 60, // seconds
		120, 300, 600, // minutes
		900, 1800, 3600,
	}

	return &collectors{
		authAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "auth_attempts_total",
				Help:      "Total number of authentication attempts",
			},
			[]string{"result"},
		),
		permissionChecks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "permission_checks_total",
				Help:      "Total number of permission checks",
			},
			[]string{"permission", "result"},
		),
		activeSessions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_sessions",
				Help:      "Number of active sessions",
			},
		),
		apiLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "api_latency_seconds",
				Help:      "API endpoint latency",
				Buckets:   buckets,
			},
			[]string{"method", "path", "status"},
		),
		vaultOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "vault_operations_total",
				Help:      "Vault operations (create, update, share) by result",
			},
			[]string{"operation", "result"},
		),
		vaultPayloadRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "vault_payload_requests_total",
				Help:      "Vault payload retrieval attempts by outcome",
			},
			[]string{"result"},
		),
		realtimeConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "realtime_connections",
				Help:      "Active realtime websocket connections",
			},
		),
		realtimeBroadcasts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "realtime_broadcasts_total",
				Help:      "Messages broadcast across realtime streams",
			},
			[]string{"stream"},
		),
		realtimeFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "realtime_failures_total",
				Help:      "Realtime broadcast or subscription failures",
			},
			[]string{"stream", "type"},
		),
		realtimeSubscriptions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "realtime_subscriptions_total",
				Help:      "Realtime subscribe/unsubscribe events",
			},
			[]string{"stream", "action"},
		),
		maintenanceRuns: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "maintenance_runs_total",
				Help:      "Maintenance job executions",
			},
			[]string{"job", "result"},
		),
		maintenanceDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "maintenance_duration_seconds",
				Help:      "Maintenance job duration",
				Buckets:   buckets,
			},
			[]string{"job"},
		),
		maintenanceLastRun: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "maintenance_last_success_timestamp",
				Help:      "Timestamp of the last successful maintenance run (seconds since epoch)",
			},
			[]string{"job"},
		),
		protocolLaunches: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "protocol_launches_total",
				Help:      "Protocol launch attempts grouped by result",
			},
			[]string{"protocol", "result"},
		),
		protocolLaunchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "protocol_launch_latency_seconds",
				Help:      "Duration to launch a protocol session",
				Buckets:   buckets,
			},
			[]string{"protocol"},
		),
		sessionDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "session_duration_seconds",
				Help:      "Observed session lifetimes",
				Buckets:   sessionBuckets,
			},
		),
		sessionShareEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "session_share_events_total",
				Help:      "Count of session sharing lifecycle events",
			},
			[]string{"event"},
		),
		sessionRecordingEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "session_recording_events_total",
				Help:      "Count of session recording lifecycle events",
			},
			[]string{"event"},
		),
	}
}

func (c *collectors) all() []prometheus.Collector {
	return []prometheus.Collector{
		c.authAttempts,
		c.permissionChecks,
		c.activeSessions,
		c.apiLatency,
		c.vaultOperations,
		c.vaultPayloadRequests,
		c.realtimeConnections,
		c.realtimeBroadcasts,
		c.realtimeFailures,
		c.realtimeSubscriptions,
		c.maintenanceRuns,
		c.maintenanceDuration,
		c.maintenanceLastRun,
		c.protocolLaunches,
		c.protocolLaunchLatency,
		c.sessionDuration,
		c.sessionShareEvents,
		c.sessionRecordingEvents,
	}
}

// observeDuration records a duration in seconds on the supplied histogram observer.
func observeDuration(observer prometheus.Observer, d time.Duration) {
	if observer == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	observer.Observe(d.Seconds())
}
