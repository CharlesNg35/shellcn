package handlers

import (
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
