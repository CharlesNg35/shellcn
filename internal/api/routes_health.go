package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/monitoring"
)

func registerHealthRoutes(r *gin.Engine, cfg *app.Config, mon *monitoring.Module) {
	if cfg == nil {
		return
	}

	if !cfg.Monitoring.Health.Enabled || mon == nil || mon.Health() == nil {
		r.GET("/health", disabledHealthHandler)
		r.GET("/health/live", disabledHealthHandler)
		r.GET("/health/ready", disabledHealthHandler)

		api := r.Group("/api")
		api.GET("/health", disabledHealthHandler)
		api.GET("/health/live", disabledHealthHandler)
		api.GET("/health/ready", disabledHealthHandler)
		return
	}

	manager := mon.Health()

	registerHealthEndpoints(r, manager)
	registerHealthEndpoints(r.Group("/api"), manager)
}

func registerHealthEndpoints(router gin.IRouter, manager *monitoring.HealthManager) {
	router.GET("/health", func(c *gin.Context) {
		report := manager.EvaluateReadiness(c.Request.Context())
		status := http.StatusOK
		if !report.Success {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"success":    report.Success,
			"status":     report.Status,
			"checked_at": time.Now().UTC(),
		})
	})

	router.GET("/health/live", func(c *gin.Context) {
		report := manager.EvaluateLiveness(c.Request.Context())
		writeHealthReport(c, report)
	})

	router.GET("/health/ready", func(c *gin.Context) {
		report := manager.EvaluateReadiness(c.Request.Context())
		writeHealthReport(c, report)
	})
}

func disabledHealthHandler(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"status":  "disabled",
	})
}

func writeHealthReport(c *gin.Context, report monitoring.HealthReport) {
	status := http.StatusOK
	if !report.Success {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{
		"success":    report.Success,
		"status":     report.Status,
		"checks":     report.Checks,
		"checked_at": time.Now().UTC(),
	})
}
