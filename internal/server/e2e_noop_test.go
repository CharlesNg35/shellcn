package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

// TestNoopEndToEnd exercises the full core path against the noop plugin:
// projection → table load via the route resolver → WS echo through the wrapper →
// audit row written → authz enforced.
func TestNoopEndToEnd(t *testing.T) {
	h := newHarness(t)

	// 1. Projection renders (what the renderer fetches on connection open).
	resp := h.do(t, http.MethodGet, "/api/plugins/noop", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("projection: want 200, got %d", resp.Status)
	}
	proj := resp.Body
	for _, want := range []string{`"name":"noop"`, `"layout":"tabs"`, `"panel":"table"`, `"panel":"terminal"`} {
		if !strings.Contains(string(proj), want) {
			t.Errorf("projection missing %q: %s", want, proj)
		}
	}

	// 2. The table panel loads via the DataSource resolver (GET list route).
	resp = h.do(t, http.MethodGet, "/api/connections/c-noop/x/noop.list", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("list: want 200, got %d", resp.Status)
	}
	list := resp.Body
	if !strings.Contains(string(list), "alpha") || !strings.Contains(string(list), `"items"`) {
		t.Errorf("list page missing data: %s", list)
	}

	// 3. WS echo through the full wrapper (ticket → upgrade → stream handler).
	tok := h.mintTicket(t, "op", "c-noop", "noop.echo", nil)
	c, err := h.dialWS(t, "op", "/api/connections/c-noop/x/noop.echo?ticket="+tok)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = c.CloseNow() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, greeting, err := c.Read(ctx)
	if err != nil || !strings.Contains(string(greeting), "noop echo ready") {
		t.Fatalf("greeting: %q err=%v", greeting, err)
	}
	if err := c.Write(ctx, websocket.MessageText, []byte("roundtrip")); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, echoed, err := c.Read(ctx)
	if err != nil || string(echoed) != "roundtrip" {
		t.Fatalf("echo: %q err=%v", echoed, err)
	}

	// 4. Audit rows were written for the list and the stream open.
	rows, _ := h.store.Audit.List(context.Background(), store.AuditFilter{ConnectionID: "c-noop"})
	events := map[string]models.AuditResult{}
	for _, r := range rows {
		events[r.Event] = r.Result
	}
	if events["noop.list"] != models.AuditAllowed {
		t.Errorf("missing allowed audit row for noop.list: %+v", rows)
	}
	if events["noop.echo"] != models.AuditAllowed {
		t.Errorf("missing allowed audit row for noop.echo: %+v", rows)
	}

	// 5. Authz is enforced: a stranger (viewer, not owner/grantee) is denied.
	if resp := h.do(t, http.MethodGet, "/api/connections/c-noop/x/noop.list", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("stranger on noop connection: want 403, got %d", resp.Status)
	}
}
