package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{deps: Deps{Logger: slog.New(slog.NewJSONHandler(&buf, nil)), AccessLog: true}}
	h := s.accessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Non-API requests (health, metrics, static) are not logged.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if buf.Len() != 0 {
		t.Fatalf("non-API request should not be logged: %s", buf.String())
	}

	// API requests are logged with method, path, and status.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/connections", nil))
	out := buf.String()
	for _, want := range []string{`"msg":"request"`, `"method":"POST"`, `"path":"/api/connections"`, `"status":204`} {
		if !strings.Contains(out, want) {
			t.Errorf("access log missing %s in: %s", want, out)
		}
	}
}
