package handlers

import (
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/monitoring"
)

// MonitoringHandler surfaces monitoring summaries for administrators.
type MonitoringHandler struct {
	module *monitoring.Module
	cfg    *app.Config
}

// NewMonitoringHandler constructs a monitoring handler. Returns nil when monitoring is disabled.
func NewMonitoringHandler(module *monitoring.Module, cfg *app.Config) *MonitoringHandler {
	if module == nil || cfg == nil {
		return nil
	}
	if !cfg.Monitoring.Health.Enabled && !cfg.Monitoring.Prometheus.Enabled {
		return nil
	}
	return &MonitoringHandler{module: module, cfg: cfg}
}

type webVitalPayload struct {
	Metric         string  `json:"metric"`
	Name           string  `json:"name"`
	Value          float64 `json:"value"`
	Rating         string  `json:"rating"`
	NavigationType string  `json:"navigation_type"`
	Delta          float64 `json:"delta"`
}

// Summary returns aggregated monitoring statistics and configuration hints.
func (h *MonitoringHandler) Summary(c *gin.Context) {
	snapshot := monitoring.Snapshot()
	endpoint := strings.TrimSpace(h.cfg.Monitoring.Prometheus.Endpoint)
	if endpoint == "" {
		endpoint = "/metrics"
	}

	response := gin.H{
		"summary": snapshot,
		"prometheus": gin.H{
			"enabled":  h.cfg.Monitoring.Prometheus.Enabled,
			"endpoint": endpoint,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// RecordVitals ingests client-side Web Vitals and forwards them to the monitoring module.
func (h *MonitoringHandler) RecordVitals(c *gin.Context) {
	if h == nil || h.module == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "monitoring module is disabled",
		})
		return
	}

	var payload struct {
		Metrics []webVitalPayload `json:"metrics"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || len(payload.Metrics) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid metrics payload",
		})
		return
	}

	accepted := 0
	for _, metric := range payload.Metrics {
		name := strings.TrimSpace(metric.Metric)
		if name == "" {
			name = strings.TrimSpace(metric.Name)
		}
		if name == "" || math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			continue
		}
		monitoring.RecordWebVital(name, metric.Rating, metric.Value)
		accepted++
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"count":   accepted,
	})
}
