package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestMonitoringHandlerRecordVitals(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	mod, err := monitoring.NewModule(monitoring.Options{})
	require.NoError(t, err)
	monitoring.SetModule(mod)

	cfg := &app.Config{
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}
	handler := NewMonitoringHandler(mod, cfg)
	require.NotNil(t, handler)

	body := `{"metrics":[{"metric":"LCP","value":1800,"rating":"good"}]}`

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req, _ := http.NewRequest(http.MethodPost, "/api/monitoring/vitals", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	handler.RecordVitals(ctx)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Contains(t, recorder.Body.String(), "\"count\":1")
}
