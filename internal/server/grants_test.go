package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestConnectionGrantUseVsManage(t *testing.T) {
	h := newHarness(t)

	// op shares c-op with viewer at `use`.
	resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant use: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	grantID := createConnID(t, resp)

	// `use` lets the grantee open/use the connection.
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Errorf("use grant should allow opening: got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections", "viewer", nil); resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"sharedWithMe":true`) ||
		!strings.Contains(string(resp.Body), `"access":"use"`) {
		t.Fatalf("shared connection list should mark grant access: status=%d body=%s", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections", "op", nil); resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"sharedByMe":true`) ||
		!strings.Contains(string(resp.Body), `"access":"owner"`) {
		t.Fatalf("owner connection list should mark shared-out state: status=%d body=%s", resp.Status, resp.Body)
	}
	// …but not edit it (edit needs manage).
	if resp := h.do(t, http.MethodPut, "/api/connections/c-op", "viewer",
		strings.NewReader(`{"name":"hax","config":{"host":"h"}}`)); resp.Status != http.StatusForbidden {
		t.Errorf("use grant must not allow edit: got %d", resp.Status)
	}

	// Revoke → access is gone immediately.
	if resp := h.do(t, http.MethodDelete, "/api/connections/c-op/grants/"+grantID, "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: want 200, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("after revoke: want 403, got %d", resp.Status)
	}

	// A `manage` grant lets the grantee edit…
	resp = h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"manage"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant manage: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPut, "/api/connections/c-op", "viewer",
		strings.NewReader(`{"name":"managed","config":{"host":"h"}}`)); resp.Status != http.StatusOK {
		t.Errorf("manage grant should allow edit: got %d (%s)", resp.Status, resp.Body)
	}
	// …and share it with another subject.
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "viewer",
		strings.NewReader(`{"subjectId":"admin","access":"use"}`)); resp.Status != http.StatusCreated {
		t.Errorf("manage grant should allow sharing: got %d", resp.Status)
	}
}

func TestGrantDeleteIsScopedToResource(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant connection: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	connGrantID := createConnID(t, resp)
	if resp := h.do(t, http.MethodDelete, "/api/connections/c-view/grants/"+connGrantID, "admin", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("delete connection grant through wrong connection: want 404, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Fatalf("connection grant should still exist, got %d", resp.Status)
	}

	credID := createCredID(t, h, "op", `{"name":"scoped","kind":"db_password","secret":"v"}`)
	resp = h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant credential: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	credGrantID := createConnID(t, resp)
	otherCredID := createCredID(t, h, "admin", `{"name":"other","kind":"db_password","secret":"v"}`)
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+otherCredID+"/grants/"+credGrantID, "admin", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("delete credential grant through wrong credential: want 404, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections", "viewer",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"`+credID+`"}}`)); resp.Status != http.StatusCreated {
		t.Fatalf("credential grant should still exist, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestCredentialGrantUse(t *testing.T) {
	h := newHarness(t)
	credID := createCredID(t, h, "op", `{"name":"shared-cred","kind":"db_password","secret":"v"}`)

	refBody := `{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"` + credID + `"}}`

	// Without a grant, viewer cannot reference op's credential.
	if resp := h.do(t, http.MethodPost, "/api/connections", "viewer", strings.NewReader(refBody)); resp.Status != http.StatusForbidden {
		t.Fatalf("reference without grant: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	// A non-owner cannot grant.
	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "viewer",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner grant: want 403, got %d", resp.Status)
	}
	// Credentials confer use only.
	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"manage"}`)); resp.Status != http.StatusBadRequest {
		t.Errorf("credential manage grant: want 400, got %d", resp.Status)
	}

	// After a use-grant, viewer can connect through it (resolution path), never reading the value.
	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`)); resp.Status != http.StatusCreated {
		t.Fatalf("grant use: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections", "viewer", strings.NewReader(refBody)); resp.Status != http.StatusCreated {
		t.Fatalf("reference with use grant: want 201, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestUserLookup(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/api/users?query=view", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("user lookup: want 200, got %d", resp.Status)
	}
	if !strings.Contains(string(resp.Body), `"viewer"`) {
		t.Errorf("lookup missing viewer: %s", resp.Body)
	}
	if strings.Contains(string(resp.Body), "password") || strings.Contains(string(resp.Body), "Hash") {
		t.Errorf("user lookup leaked sensitive fields: %s", resp.Body)
	}
}
