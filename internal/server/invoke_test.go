package server_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func (h *harness) user(t *testing.T, id string) models.User {
	t.Helper()
	u, err := h.store.Users.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get user %q: %v", id, err)
	}
	return u
}

func auditRows(t *testing.T, h *harness, connID string) []models.AuditEntry {
	t.Helper()
	rows, err := h.store.Audit.List(context.Background(), store.AuditFilter{ConnectionID: connID})
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	return rows
}

// InvokeRoute is the shared secure pipeline the AI agent uses. These tests pin
// parity with HTTP dispatch: authz allow/deny, schema validation, handler error,
// audit recording, and the ai source marker.

func TestInvokeRouteAllowedRecordsAudit(t *testing.T) {
	h := newHarness(t)
	ctx := audit.WithSource(context.Background(), audit.SourceAI, "turn-7")

	result, err := h.srv.InvokeRoute(ctx, h.user(t, "op"), "c-op", "tester.list", nil, nil)
	if err != nil {
		t.Fatalf("invoke tester.list: %v", err)
	}
	page, ok := result.(plugin.Page[string])
	if !ok || len(page.Items) != 2 {
		t.Fatalf("unexpected result %#v", result)
	}

	var found bool
	for _, r := range auditRows(t, h, "c-op") {
		if r.RouteID == "tester.list" && r.Result == models.AuditAllowed {
			found = true
			if r.Source != audit.SourceAI || r.TurnID != "turn-7" {
				t.Fatalf("audit source/turn = %q/%q, want ai/turn-7", r.Source, r.TurnID)
			}
		}
	}
	if !found {
		t.Fatal("missing allowed audit row for tester.list")
	}
}

func TestInvokeRouteDeniedForStranger(t *testing.T) {
	h := newHarness(t)
	// viewer is neither owner nor grantee of c-op → forbidden, audited denied.
	_, err := h.srv.InvokeRoute(context.Background(), h.user(t, "viewer"), "c-op", "tester.list", nil, nil)
	if !errors.Is(err, policy.ErrForbidden) {
		t.Fatalf("stranger invoke: want ErrForbidden, got %v", err)
	}
	for _, r := range auditRows(t, h, "c-op") {
		if r.RouteID == "tester.list" && r.Result == models.AuditDenied {
			return
		}
	}
	t.Fatal("missing denied audit row")
}

func TestInvokeRouteRBACBlocksRiskBeyondRole(t *testing.T) {
	h := newHarness(t)
	// viewer owns c-view but the viewer role cannot perform a destructive route,
	// exactly as over HTTP — the agent cannot exceed the user's RBAC.
	_, err := h.srv.InvokeRoute(context.Background(), h.user(t, "viewer"), "c-view", "tester.danger", nil, nil)
	if !errors.Is(err, policy.ErrForbidden) {
		t.Fatalf("viewer destructive: want ErrForbidden, got %v", err)
	}
}

func TestInvokeRouteValidationFailureSkipsHandler(t *testing.T) {
	h := newHarness(t)
	schemaOnlyCalls.Store(0)

	_, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.schema", nil, []byte(`{}`))
	if err == nil {
		t.Fatal("invalid input: want error")
	}
	if got := schemaOnlyCalls.Load(); got != 0 {
		t.Fatalf("handler ran despite invalid input: calls=%d", got)
	}

	if _, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.schema", nil, []byte(`{"name":"release"}`)); err != nil {
		t.Fatalf("valid input: %v", err)
	}
	if got := schemaOnlyCalls.Load(); got != 1 {
		t.Fatalf("handler call count = %d, want 1", got)
	}
}

func TestInvokeRouteHandlerErrorAudited(t *testing.T) {
	h := newHarness(t)
	_, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.unauth", nil, nil)
	if !errors.Is(err, plugin.ErrUnauthorized) {
		t.Fatalf("handler error: want ErrUnauthorized, got %v", err)
	}
	for _, r := range auditRows(t, h, "c-op") {
		if r.RouteID == "tester.unauth" && r.Result == models.AuditError {
			return
		}
	}
	t.Fatal("missing error audit row for tester.unauth")
}

func TestInvokeRouteResolvesPathParams(t *testing.T) {
	h := newHarness(t)
	result, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.echoparam", map[string]string{"name": "resolved"}, nil)
	if err != nil {
		t.Fatalf("invoke echoparam: %v", err)
	}
	if m, ok := result.(map[string]string); !ok || m["name"] != "resolved" {
		t.Fatalf("unexpected result %#v", result)
	}

	if _, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.echoparam", nil, nil); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("missing required param: want ErrInvalidInput, got %v", err)
	}
}

func TestInvokeRouteRejectsStreamRoutes(t *testing.T) {
	h := newHarness(t)
	if _, err := h.srv.InvokeRoute(context.Background(), h.user(t, "op"), "c-op", "tester.ws", nil, nil); !errors.Is(err, plugin.ErrNotSupported) {
		t.Fatalf("stream route: want ErrNotSupported, got %v", err)
	}
}

func TestHTTPDispatchRecordsHTTPSource(t *testing.T) {
	h := newHarness(t)
	if resp := h.do(t, "GET", "/api/connections/c-op/x/tester.list", "op", nil); resp.Status != 200 {
		t.Fatalf("want 200, got %d", resp.Status)
	}
	for _, r := range auditRows(t, h, "c-op") {
		if r.RouteID == "tester.list" && r.Result == models.AuditAllowed {
			if r.Source != audit.SourceHTTP {
				t.Fatalf("http audit source = %q, want http", r.Source)
			}
			return
		}
	}
	t.Fatal("missing allowed audit row")
}
