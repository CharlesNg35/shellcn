package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAdminUserDetailEndpoints(t *testing.T) {
	h := newHarness(t)

	// Generate an audit entry owned by op.
	h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "op", nil)

	// Single user.
	if resp := h.do(t, http.MethodGet, "/api/admin/users/op", "admin", nil); resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"username":"op"`) {
		t.Fatalf("get user: status=%d body=%s", resp.Status, resp.Body)
	}

	// Connections inventory: metadata only — name/protocol, never config/secrets.
	resp := h.do(t, http.MethodGet, "/api/admin/users/op/connections", "admin", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"protocol":"tester"`) {
		t.Fatalf("user connections: status=%d body=%s", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), `"config"`) || strings.Contains(string(resp.Body), `"secret`) {
		t.Fatalf("user connections leaked config/secrets: %s", resp.Body)
	}

	// Paginated audit for the user.
	resp = h.do(t, http.MethodGet, "/api/admin/users/op/audit?limit=10&offset=0", "admin", nil)
	if resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"items"`) ||
		!strings.Contains(string(resp.Body), `"total"`) {
		t.Fatalf("user audit: status=%d body=%s", resp.Status, resp.Body)
	}

	// Non-admins cannot reach the admin user-detail endpoints.
	for _, path := range []string{
		"/api/admin/users/op",
		"/api/admin/users/op/connections",
		"/api/admin/users/op/audit",
	} {
		if resp := h.do(t, http.MethodGet, path, "op", nil); resp.Status != http.StatusForbidden {
			t.Errorf("non-admin %s: want 403, got %d", path, resp.Status)
		}
	}
}
