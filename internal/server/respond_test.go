package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestWriteErrorAgentUnavailable(t *testing.T) {
	// Mirrors the real chain: an HTTP-client dial failure through a dead tunnel.
	dial := fmt.Errorf("%w: session shutdown", transport.ErrAgentUnavailable)
	err := fmt.Errorf(`Get "http://shellcn-agent.internal/disks": %w`, dial)

	rec := httptest.NewRecorder()
	writeError(rec, nil, err)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: want 503, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "offline") {
		t.Errorf("want a clear offline message, got %s", body)
	}
	if strings.Contains(body, "agent.internal") || strings.Contains(body, "session shutdown") {
		t.Errorf("internal detail leaked to client: %s", body)
	}
}

func TestWriteErrorNotSupportedCarriesMessage(t *testing.T) {
	// ErrNotSupported is a client-actionable state (501), so its message must
	// reach the user rather than the bare "Not Implemented" status text.
	err := fmt.Errorf("%w: this container has no shell (e.g. distroless)", plugin.ErrNotSupported)
	rec := httptest.NewRecorder()
	writeError(rec, nil, err)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status: want 501, got %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "distroless") {
		t.Errorf("want the plugin message, got %s", body)
	}
}

func TestWriteErrorServerFaultHidesDetail(t *testing.T) {
	// A genuine 500 must not leak its detail.
	rec := httptest.NewRecorder()
	writeError(rec, nil, fmt.Errorf("boom: secret internal detail"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret internal detail") {
		t.Errorf("internal detail leaked: %s", body)
	}
}
