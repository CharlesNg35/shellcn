// Package telemetry provides structured logging, Prometheus metrics, and health
// checks so the gateway itself is observable from day one.
package telemetry

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the gateway's Prometheus collectors on a private registry.
type Metrics struct {
	reg *prometheus.Registry

	sessionsOpen    prometheus.Gauge
	channelsOpen    prometheus.Gauge
	wsConnections   prometheus.Gauge
	actionLatency   *prometheus.HistogramVec
	authzFailures   prometheus.Counter
	secretAccess    prometheus.Counter
	pluginHealth    *prometheus.GaugeVec
	recordingsOpen  prometheus.Gauge
	recordingBytes  prometheus.Counter
	recordingFailed prometheus.Counter
}

// NewMetrics registers the collectors on a fresh registry.
func NewMetrics() *Metrics {
	m := &Metrics{
		reg:           prometheus.NewRegistry(),
		sessionsOpen:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "shellcn_sessions_open", Help: "Open upstream sessions."}),
		channelsOpen:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "shellcn_channels_open", Help: "Open tracked channels."}),
		wsConnections: prometheus.NewGauge(prometheus.GaugeOpts{Name: "shellcn_ws_connections", Help: "Active WebSocket connections."}),
		actionLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "shellcn_action_duration_seconds",
			Help:    "Route handler latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"risk", "result"}),
		authzFailures:   prometheus.NewCounter(prometheus.CounterOpts{Name: "shellcn_authz_failures_total", Help: "Failed authorizations."}),
		secretAccess:    prometheus.NewCounter(prometheus.CounterOpts{Name: "shellcn_secret_access_total", Help: "Secret decryptions."}),
		pluginHealth:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "shellcn_plugin_healthy", Help: "1 if a plugin's last health check passed, else 0."}, []string{"plugin"}),
		recordingsOpen:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "shellcn_recordings_active", Help: "Active session recordings."}),
		recordingBytes:  prometheus.NewCounter(prometheus.CounterOpts{Name: "shellcn_recording_bytes_total", Help: "Bytes written to recordings."}),
		recordingFailed: prometheus.NewCounter(prometheus.CounterOpts{Name: "shellcn_recording_failures_total", Help: "Recordings that failed to capture."}),
	}
	m.reg.MustRegister(
		m.sessionsOpen, m.channelsOpen, m.wsConnections,
		m.actionLatency, m.authzFailures, m.secretAccess, m.pluginHealth,
		m.recordingsOpen, m.recordingBytes, m.recordingFailed,
	)
	return m
}

// Handler serves the metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// Registry exposes the underlying registry (for tests or extra collectors).
func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

// SetSessions / SetChannels reflect the live registry counts.
func (m *Metrics) SetSessions(n int) { m.sessionsOpen.Set(float64(n)) }
func (m *Metrics) SetChannels(n int) { m.channelsOpen.Set(float64(n)) }

// WSOpened / WSClosed track active WebSocket connections.
func (m *Metrics) WSOpened() { m.wsConnections.Inc() }
func (m *Metrics) WSClosed() { m.wsConnections.Dec() }

// ObserveAction records a handler's latency labelled by risk + result.
func (m *Metrics) ObserveAction(risk, result string, d time.Duration) {
	m.actionLatency.WithLabelValues(risk, result).Observe(d.Seconds())
}

// IncAuthzFailure counts a denied authorization.
func (m *Metrics) IncAuthzFailure() { m.authzFailures.Inc() }

// IncSecretAccess counts a secret decryption.
func (m *Metrics) IncSecretAccess() { m.secretAccess.Inc() }

// RecordingStarted / RecordingFinished track active recordings.
func (m *Metrics) RecordingStarted()  { m.recordingsOpen.Inc() }
func (m *Metrics) RecordingFinished() { m.recordingsOpen.Dec() }

// AddRecordingBytes counts bytes written to recordings.
func (m *Metrics) AddRecordingBytes(n int) { m.recordingBytes.Add(float64(n)) }

// RecordingFailed counts a recording that failed to capture.
func (m *Metrics) RecordingFailed() { m.recordingFailed.Inc() }

// SetPluginHealth records a plugin's latest health state.
func (m *Metrics) SetPluginHealth(plugin string, healthy bool) {
	v := 0.0
	if healthy {
		v = 1
	}
	m.pluginHealth.WithLabelValues(plugin).Set(v)
}
