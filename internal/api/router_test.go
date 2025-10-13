package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	testutil "github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/services"
)

const testVaultKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRouter_PublicAndProtectedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Open in-memory DB and run migrations/seed
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	seedProtocol(t, db)
	seedProtocol(t, db)
	seedProtocol(t, db)
	seedProtocol(t, db)
	seedProtocol(t, db)
	seedProtocol(t, db)
	seedProtocol(t, db)

	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "test-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	cfg := &app.Config{
		Vault: app.VaultConfig{
			EncryptionKey: testVaultKeyHex,
		},
		Auth: app.AuthConfig{
			JWT: app.JWTSettings{
				Secret: "test-secret",
				Issuer: "test",
				TTL:    time.Hour,
			},
			Session: app.SessionSettings{
				RefreshTTL:    24 * time.Hour,
				RefreshLength: 48,
			},
		},
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	if err != nil {
		t.Fatalf("session service: %v", err)
	}
	protocolSvc, err := services.NewProtocolService(db, nil)
	require.NoError(t, err)
	protocols, err := protocolSvc.ListAll(context.Background())
	require.NoError(t, err)
	if len(protocols) == 0 {
		t.Fatalf("expected seeded protocols")
	}

	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		t.Fatalf("monitoring module: %v", err)
	}
	monitoring.SetModule(mon)
	report := mon.Health().EvaluateReadiness(context.Background())
	if !report.Success {
		t.Logf("initial readiness: %+v", report)
	}

	router, err := NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore(), mon)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	report = mon.Health().EvaluateReadiness(context.Background())
	if !report.Success {
		t.Logf("post-router readiness: %+v", report)
	}

	// Health should be public
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for /health, got %d", w.Code)
	}

	// Protected endpoint without auth should be 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/auth/me", nil)
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 for /api/auth/me without token, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/users", nil)
	router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("expected 401 for /api/users without token, got %d", w.Code)
	}
}

func TestRouter_MetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "metrics-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	cfg := &app.Config{
		Vault: app.VaultConfig{
			EncryptionKey: testVaultKeyHex,
		},
		Auth: app.AuthConfig{
			JWT: app.JWTSettings{
				Secret: "metrics-secret",
				Issuer: "test",
				TTL:    time.Hour,
			},
			Session: app.SessionSettings{
				RefreshTTL:    24 * time.Hour,
				RefreshLength: 48,
			},
		},
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		t.Fatalf("monitoring module: %v", err)
	}
	monitoring.SetModule(mon)

	router, err := NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore(), mon)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	report := mon.Health().EvaluateReadiness(context.Background())
	if !report.Success {
		t.Fatalf("readiness failed: %+v", report)
	}

	// Trigger a request to generate metrics
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}

	metricsRec := httptest.NewRecorder()
	metricsReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d", metricsRec.Code)
	}

	body := metricsRec.Body.String()
	if !strings.Contains(body, `shellcn_api_latency_seconds_count{method="GET",path="health",status="200"}`) {
		t.Fatalf("metrics output missing latency series: %s", body)
	}
}

func TestRouter_MetricsCustomEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "custom-metrics", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	cfg := &app.Config{
		Vault: app.VaultConfig{EncryptionKey: testVaultKeyHex},
		Auth: app.AuthConfig{
			JWT:     app.JWTSettings{Secret: "custom-metrics", Issuer: "test", TTL: time.Hour},
			Session: app.SessionSettings{RefreshTTL: 24 * time.Hour, RefreshLength: 48},
		},
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/custom-metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		t.Fatalf("monitoring module: %v", err)
	}
	monitoring.SetModule(mon)

	router, err := NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore(), mon)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	metricsRec := httptest.NewRecorder()
	metricsReq, _ := http.NewRequest(http.MethodGet, "/custom-metrics", nil)
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for custom metrics path, got %d", metricsRec.Code)
	}

	defaultRec := httptest.NewRecorder()
	defaultReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for default metrics path, got %d", defaultRec.Code)
	}
}

func TestRouter_MetricsDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "metrics-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	cfg := &app.Config{
		Vault: app.VaultConfig{
			EncryptionKey: testVaultKeyHex,
		},
		Auth: app.AuthConfig{
			JWT: app.JWTSettings{
				Secret: "metrics-secret",
				Issuer: "test",
				TTL:    time.Hour,
			},
			Session: app.SessionSettings{
				RefreshTTL:    24 * time.Hour,
				RefreshLength: 48,
			},
		},
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: false, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: true},
		},
	}
	cfg.Monitoring.Prometheus.Enabled = false

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		t.Fatalf("monitoring module: %v", err)
	}
	monitoring.SetModule(mon)

	router, err := NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore(), mon)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	metricsRec := httptest.NewRecorder()
	metricsReq, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for disabled metrics, got %d", metricsRec.Code)
	}
}

func TestRouter_HealthDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())
	jwtSvc, err := iauth.NewJWTService(iauth.JWTConfig{Secret: "health-secret", Issuer: "test", AccessTokenTTL: 900000000000})
	if err != nil {
		t.Fatalf("jwt service: %v", err)
	}

	cfg := &app.Config{
		Vault: app.VaultConfig{EncryptionKey: testVaultKeyHex},
		Auth: app.AuthConfig{
			JWT:     app.JWTSettings{Secret: "health-secret", Issuer: "test", TTL: time.Hour},
			Session: app.SessionSettings{RefreshTTL: 24 * time.Hour, RefreshLength: 48},
		},
		Monitoring: app.MonitoringConfig{
			Prometheus: app.PrometheusConfig{Enabled: true, Endpoint: "/metrics"},
			Health:     app.HealthConfig{Enabled: false},
		},
	}
	cfg.Monitoring.Health.Enabled = false

	sessionSvc, err := iauth.NewSessionService(db, jwtSvc, cfg.Auth.SessionServiceConfig())
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	mon, err := monitoring.NewModule(monitoring.Options{})
	if err != nil {
		t.Fatalf("monitoring module: %v", err)
	}
	monitoring.SetModule(mon)

	router, err := NewRouter(db, jwtSvc, cfg, sessionSvc, middleware.NewMemoryRateStore(), mon)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when health disabled, got %d", w.Code)
	}
}

func seedProtocol(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.ConnectionProtocol{}).Count(&count).Error)
	if count > 0 {
		return
	}
	entry := models.ConnectionProtocol{
		Name:          "SSH",
		ProtocolID:    "ssh",
		DriverID:      "ssh",
		Module:        "ssh",
		Icon:          "terminal",
		Category:      "terminal",
		Description:   "Seed protocol",
		DefaultPort:   22,
		SortOrder:     1,
		DriverEnabled: true,
		ConfigEnabled: true,
	}
	entry.Features = datatypes.JSON([]byte("[]"))
	entry.Capabilities = datatypes.JSON([]byte(`{"terminal":true}`))
	require.NoError(t, db.Create(&entry).Error)
	require.NoError(t, db.Model(&models.ConnectionProtocol{}).Count(&count).Error)
	if count == 0 {
		t.Fatalf("protocol seed failed")
	}
}
