package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
)

func TestConnectionGrantUseVsManage(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"use"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant use: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	grantID := createConnID(t, resp)

	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Errorf("use grant should allow opening: got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections", "viewer", nil); resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"sharedWithMe":true`) ||
		!strings.Contains(string(resp.Body), `"access":"use"`) ||
		!strings.Contains(string(resp.Body), `"canShare":false`) ||
		!strings.Contains(string(resp.Body), `"ownerName":`) {
		t.Fatalf("shared connection list should mark grant access + owner: status=%d body=%s", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections", "op", nil); resp.Status != http.StatusOK ||
		!strings.Contains(string(resp.Body), `"sharedByMe":true`) ||
		!strings.Contains(string(resp.Body), `"access":"owner"`) ||
		!strings.Contains(string(resp.Body), `"canShare":true`) {
		t.Fatalf("owner connection list should mark shared-out state: status=%d body=%s", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPut, "/api/connections/c-op", "viewer",
		strings.NewReader(`{"name":"hax","config":{"host":"h"}}`)); resp.Status != http.StatusForbidden {
		t.Errorf("use grant must not allow edit: got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodDelete, "/api/connections/c-op/grants/"+grantID, "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("revoke: want 200, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("after revoke: want 403, got %d", resp.Status)
	}

	resp = h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"manage"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant manage: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPut, "/api/connections/c-op", "viewer",
		strings.NewReader(`{"name":"managed","config":{"host":"h"}}`)); resp.Status != http.StatusOK {
		t.Errorf("manage grant should allow edit: got %d (%s)", resp.Status, resp.Body)
	}
	// Only the owner may share, never a manage-grantee or admin.
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "viewer",
		strings.NewReader(`{"subjectId":"admin","access":"use"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("manage grant must not allow re-sharing: got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "admin",
		strings.NewReader(`{"subjectId":"admin","access":"use"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("admin must not share another's connection: got %d", resp.Status)
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
	// viewer owns c-view; the grant belongs to c-op, so it is not found there.
	if resp := h.do(t, http.MethodDelete, "/api/connections/c-view/grants/"+connGrantID, "viewer", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("delete connection grant through wrong connection: want 404, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Fatalf("connection grant should still exist, got %d", resp.Status)
	}

	credID := createCredID(t, h, "op", `{"name":"scoped","kind":"db_password","secret":"v"}`)
	resp = h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"op2","access":"use"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("grant credential: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	credGrantID := createConnID(t, resp)
	otherCredID := createCredID(t, h, "admin", `{"name":"other","kind":"db_password","secret":"v"}`)
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+otherCredID+"/grants/"+credGrantID, "admin", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("delete credential grant through wrong credential: want 404, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections", "op2",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"`+credID+`"}}`)); resp.Status != http.StatusCreated {
		t.Fatalf("credential grant should still exist, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestCredentialGrantUse(t *testing.T) {
	h := newHarness(t)
	credID := createCredID(t, h, "op", `{"name":"shared-cred","kind":"db_password","secret":"v"}`)

	refBody := `{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"` + credID + `"}}`

	if resp := h.do(t, http.MethodPost, "/api/connections", "op2", strings.NewReader(refBody)); resp.Status != http.StatusForbidden {
		t.Fatalf("reference without grant: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op2",
		strings.NewReader(`{"subjectId":"op2","access":"use"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner grant: want 403, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"op2","access":"manage"}`)); resp.Status != http.StatusBadRequest {
		t.Errorf("credential manage grant: want 400, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodPost, "/api/credentials/"+credID+"/grants", "op",
		strings.NewReader(`{"subjectId":"op2","access":"use"}`)); resp.Status != http.StatusCreated {
		t.Fatalf("grant use: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections", "op2", strings.NewReader(refBody)); resp.Status != http.StatusCreated {
		t.Fatalf("reference with use grant: want 201, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestUserLookupIsAdminOnly(t *testing.T) {
	h := newHarness(t)
	if resp := h.do(t, http.MethodGet, "/api/admin/users/search?query=view", "op", nil); resp.Status != http.StatusForbidden {
		t.Errorf("operator user search: want 403, got %d", resp.Status)
	}
	resp := h.do(t, http.MethodGet, "/api/admin/users/search?query=view", "admin", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"viewer"`) {
		t.Fatalf("admin user lookup: status=%d body=%s", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "password") || strings.Contains(string(resp.Body), "Hash") {
		t.Errorf("user lookup leaked sensitive fields: %s", resp.Body)
	}
}

func TestShareByEmail(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_ = h.store.Users.Update(ctx, &models.User{ID: "viewer", Username: "viewer", Email: "viewer@example.com", Roles: []models.Role{models.RoleViewer}})

	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"email":"viewer@example.com","access":"use"}`)); resp.Status != http.StatusCreated {
		t.Fatalf("share by email: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/t.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Errorf("grantee open after email share: want 200, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"email":"nobody@example.com","access":"use"}`)); resp.Status != http.StatusNotFound {
		t.Errorf("share to unknown email: want 404, got %d", resp.Status)
	}
}
