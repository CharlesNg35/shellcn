package telemetry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/telemetry"
)

func TestMetricsExposed(t *testing.T) {
	m := telemetry.NewMetrics()
	m.SetSessions(3)
	m.SetChannels(5)
	m.ObserveAction("write", "allowed", 12*time.Millisecond)
	m.IncAuthzFailure()
	m.IncSecretAccess()

	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		"shellcn_sessions_open 3",
		"shellcn_channels_open 5",
		"shellcn_authz_failures_total 1",
		"shellcn_secret_access_total 1",
		"shellcn_action_duration_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}

func TestHealthHandler(t *testing.T) {
	h := telemetry.NewHealth()
	h.Register("store", func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("healthy: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("body: %s", rec.Body.String())
	}

	h.Register("broken", func(context.Context) error { return errors.New("down") })
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("degraded: want 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "down") {
		t.Errorf("degraded body should name the failing check: %s", rec.Body.String())
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	var seen string
	h := telemetry.RequestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = telemetry.RequestID(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if seen == "" {
		t.Error("request id not set in context")
	}
	if rec.Header().Get(telemetry.RequestIDHeader) != seen {
		t.Error("response header request id should match context")
	}

	// An incoming id is preserved.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(telemetry.RequestIDHeader, "abc123")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen != "abc123" {
		t.Errorf("incoming request id not preserved: %q", seen)
	}
}

func TestLoggerWrites(_ *testing.T) {
	// Smoke: the logger constructs and logs without panicking.
	log := telemetry.NewLogger(0, true)
	log.Info("hello", "k", "v")
}
