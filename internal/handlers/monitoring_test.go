package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/monitoring"
)

func TestMonitoringHandlerSummary(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mod, err := monitoring.NewModule(monitoring.Options{})
	require.NoError(t, err)
	monitoring.SetModule(mod)

	monitoring.RecordAuthAttempt("success")
	monitoring.RecordMaintenanceRun("session_cleanup", "success", "", 200*time.Millisecond)

	cfg := &app.Config{
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}
	handler := NewMonitoringHandler(mod, cfg)
	require.NotNil(t, handler)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request, _ = http.NewRequest(http.MethodGet, "/api/monitoring/summary", nil)

	handler.Summary(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "\"success\":true")
}
