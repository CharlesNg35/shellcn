package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/transport"
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
