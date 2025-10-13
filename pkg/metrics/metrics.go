package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AuthAttempts records authentication attempts by result (success|failure).
	AuthAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shellcn_auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"result"},
	)

	// PermissionChecks counts permission evaluations and their outcome (allow|deny|error).
	PermissionChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shellcn_permission_checks_total",
			Help: "Total number of permission checks",
		},
		[]string{"permission", "result"},
	)

	// ActiveSessions tracks active sessions (not expired/revoked).
	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "shellcn_active_sessions",
			Help: "Number of active sessions",
		},
	)

	// APILatency measures HTTP request latencies.
	APILatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shellcn_api_latency_seconds",
			Help:    "API endpoint latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// VaultOperations tracks vault service operations by action and result.
	VaultOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shellcn_vault_operations_total",
			Help: "Vault operations (create, update, share) by result",
		},
		[]string{"operation", "result"},
	)

	// VaultPayloadRequests counts payload retrieval attempts by outcome.
	VaultPayloadRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shellcn_vault_payload_requests_total",
			Help: "Vault payload retrieval attempts by outcome",
		},
		[]string{"result"},
	)
)
