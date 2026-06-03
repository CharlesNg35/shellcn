package server_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type protocolItem struct {
	Name         string   `json:"name"`
	External     bool     `json:"external"`
	Healthy      bool     `json:"healthy"`
	Availability string   `json:"availability"`
	Risks        []string `json:"risks"`
	Recording    []string `json:"recording"`
}

func protocolItems(t *testing.T, body []byte) map[string]protocolItem {
	t.Helper()
	var payload struct {
		Protocols []protocolItem `json:"protocols"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v (%s)", err, body)
	}
	out := map[string]protocolItem{}
	for _, it := range payload.Protocols {
		out[it.Name] = it
	}
	return out
}

func TestAdminProtocolsRequiresAdmin(t *testing.T) {
	h := newHarness(t)
	if resp := h.do(t, http.MethodGet, "/api/admin/protocols", "op", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("non-admin list: status %d", resp.Status)
	}
	resp := h.do(t, http.MethodGet, "/api/admin/protocols", "admin", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("admin list: status %d (%s)", resp.Status, resp.Body)
	}
	items := protocolItems(t, resp.Body)
	tester, ok := items["tester"]
	if !ok {
		t.Fatal("tester missing from admin list")
	}
	if tester.Availability != "enabled" {
		t.Fatalf("tester default availability = %q, want enabled", tester.Availability)
	}
	// The capability surface is projected for review.
	if len(tester.Risks) == 0 {
		t.Error("expected tester to expose route risks")
	}
	if !slices.Contains(tester.Recording, "terminal") {
		t.Errorf("expected tester recording to include terminal, got %v", tester.Recording)
	}
}

func setAvailability(t *testing.T, h *harness, name, state string) {
	t.Helper()
	body := strings.NewReader(`{"availability":"` + state + `"}`)
	resp := h.do(t, http.MethodPut, "/api/admin/protocols/"+name, "admin", body)
	if resp.Status != http.StatusNoContent {
		t.Fatalf("set %s=%s: status %d (%s)", name, state, resp.Status, resp.Body)
	}
}

func TestProtocolDisabledHiddenAndBlocked(t *testing.T) {
	h := newHarness(t)
	setAvailability(t, h, "tester", "disabled")

	// Hidden from the catalog for everyone, including admins.
	for _, user := range []string{"op", "admin"} {
		resp := h.do(t, http.MethodGet, "/api/plugins", user, nil)
		if strings.Contains(string(resp.Body), `"tester"`) {
			t.Fatalf("disabled protocol still listed for %s: %s", user, resp.Body)
		}
	}

	// Connecting an existing connection of the disabled protocol fails clearly.
	resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "op", nil)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("connect disabled: status %d (%s)", resp.Status, resp.Body)
	}
	if !strings.Contains(string(resp.Body), "not available") {
		t.Fatalf("expected a 'not available' message, got %s", resp.Body)
	}

	// Re-enabling restores access.
	setAvailability(t, h, "tester", "enabled")
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("connect re-enabled: status %d (%s)", resp.Status, resp.Body)
	}
}

func TestProtocolAdminOnlyVisibility(t *testing.T) {
	h := newHarness(t)
	setAvailability(t, h, "tester", "admin_only")

	opResp := h.do(t, http.MethodGet, "/api/plugins", "op", nil)
	if strings.Contains(string(opResp.Body), `"tester"`) {
		t.Fatalf("admin_only protocol listed for non-admin: %s", opResp.Body)
	}
	adminResp := h.do(t, http.MethodGet, "/api/plugins", "admin", nil)
	if !strings.Contains(string(adminResp.Body), `"tester"`) {
		t.Fatalf("admin_only protocol missing for admin: %s", adminResp.Body)
	}

	// A non-admin cannot open a session for it; an admin can (subject to RBAC).
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "op", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("admin_only connect by non-admin: status %d (%s)", resp.Status, resp.Body)
	}
}
