package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func createCredID(t *testing.T, h *harness, userID, body string) string {
	t.Helper()
	resp := h.do(t, http.MethodPost, "/api/credentials", userID, strings.NewReader(body))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create credential: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "secret") && strings.Contains(string(resp.Body), "value") {
		t.Fatalf("credential summary leaked secret material: %s", resp.Body)
	}
	return createConnID(t, resp) // reuses the {"id":...} extractor
}

func TestCredentialCreateRotateAuthz(t *testing.T) {
	h := newHarness(t)

	// op creates a credential; the response is a summary with no secret value.
	id := createCredID(t, h, "op",
		`{"name":"db pw","kind":"db_password","username":"app","secret":"secret-value-123"}`)

	// op (owner) may rotate it.
	resp := h.do(t, http.MethodPut, "/api/credentials/"+id, "op",
		strings.NewReader(`{"name":"db pw","kind":"db_password","username":"app","secret":"rotated-456"}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("owner rotate: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "rotated-456") {
		t.Fatalf("rotate response leaked secret: %s", resp.Body)
	}

	// viewer (not owner, not admin) may neither rotate nor delete.
	if resp := h.do(t, http.MethodPut, "/api/credentials/"+id, "viewer",
		strings.NewReader(`{"name":"x","kind":"db_password"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner rotate: want 403, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+id, "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner delete: want 403, got %d", resp.Status)
	}

	// admin may delete it.
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+id, "admin", nil); resp.Status != http.StatusOK {
		t.Errorf("admin delete: want 200, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestCredentialDeleteBlockedWhileReferenced(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	credID := createCredID(t, h, "op",
		`{"name":"shared","kind":"db_password","secret":"v"}`)

	// A connection references it → deletion is blocked with 409.
	connResp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"uses-cred","protocol":"tester","config":{"host":"h","credential_id":"`+credID+`"}}`))
	connID := createConnID(t, connResp)

	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+credID, "op", nil); resp.Status != http.StatusConflict {
		t.Fatalf("delete while referenced: want 409, got %d (%s)", resp.Status, resp.Body)
	}

	// Once the referencing connection is gone, deletion succeeds.
	if resp := h.do(t, http.MethodDelete, "/api/connections/"+connID, "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("delete connection: want 200, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+credID, "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("delete after unreferenced: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if _, err := h.store.Credentials.Get(ctx, credID); err == nil {
		t.Error("credential should be deleted")
	}
}

func TestCredentialDeleteBlockedForAnyCredentialRefField(t *testing.T) {
	h := newHarness(t)

	credID := createCredID(t, h, "op",
		`{"name":"api token","kind":"api_token","secret":"v"}`)

	connResp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"uses-alt","protocol":"tester","config":{"host":"h","api_credential":"`+credID+`"}}`))
	if connResp.Status != http.StatusCreated {
		t.Fatalf("create connection with alternate credential ref: want 201, got %d (%s)", connResp.Status, connResp.Body)
	}

	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+credID, "op", nil); resp.Status != http.StatusConflict {
		t.Fatalf("delete while referenced through alternate field: want 409, got %d (%s)", resp.Status, resp.Body)
	}
}
