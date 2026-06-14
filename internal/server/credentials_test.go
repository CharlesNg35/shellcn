package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
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
		`{"name":"db pw","kind":"db_password","values":{"username":"app","password":"secret-value-123"}}`)

	// op (owner) may rotate it.
	resp := h.do(t, http.MethodPut, "/api/credentials/"+id, "op",
		strings.NewReader(`{"name":"db pw","kind":"db_password","values":{"username":"app","password":"rotated-456"}}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("owner rotate: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "rotated-456") {
		t.Fatalf("rotate response leaked secret: %s", resp.Body)
	}

	// viewer (not owner, not admin) may neither rotate nor delete.
	if resp := h.do(t, http.MethodPut, "/api/credentials/"+id, "viewer",
		strings.NewReader(`{"name":"x","kind":"db_password","values":{"username":"app"}}`)); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner rotate: want 403, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+id, "viewer", nil); resp.Status != http.StatusForbidden {
		t.Errorf("non-owner delete: want 403, got %d", resp.Status)
	}

	// admin has no implicit access — it may neither rotate nor delete another's.
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+id, "admin", nil); resp.Status != http.StatusForbidden {
		t.Errorf("admin delete of another's credential: want 403, got %d", resp.Status)
	}
	// the owner may delete it.
	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+id, "op", nil); resp.Status != http.StatusOK {
		t.Errorf("owner delete: want 200, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestCredentialKindsEndpoint(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/credential-kinds", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("credential kinds: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	var out []plugin.CredentialKindInfo
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("decode credential kinds: %v", err)
	}
	seen := map[plugin.CredentialKind]bool{}
	var sshPassword plugin.CredentialKindInfo
	for _, kind := range out {
		seen[kind.Kind] = true
		if kind.Kind == plugin.CredentialSSHPassword {
			sshPassword = kind
		}
		if kind.Label == "" || len(kind.Fields) == 0 {
			t.Fatalf("credential kind missing labels: %+v", kind)
		}
	}
	if !seen[plugin.CredentialDBPassword] || !seen[plugin.CredentialSSHPassword] {
		t.Fatalf("credential catalog missing expected kinds: %+v", seen)
	}
	if len(sshPassword.CompatibleProtocols) != 1 || sshPassword.CompatibleProtocols[0] != "ssh" {
		t.Fatalf("ssh compatible protocols = %+v, want [ssh]", sshPassword.CompatibleProtocols)
	}
}

func TestCredentialCreateRejectsUnknownAndDerivesProtocols(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/credentials", "op",
		strings.NewReader(`{"name":"db","kind":"db_password","protocols":["ssh"],"values":{"username":"app","password":"x"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create with ignored manual protocols: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	var created struct {
		Protocols []string `json:"protocols"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil {
		t.Fatalf("decode created credential: %v", err)
	}
	if len(created.Protocols) != 1 || created.Protocols[0] != "tester" {
		t.Fatalf("created protocols = %+v, want derived [tester]", created.Protocols)
	}

	resp = h.do(t, http.MethodPost, "/api/credentials", "op",
		strings.NewReader(`{"name":"bad","kind":"made_up","values":{"password":"x"}}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("unknown kind: want 400, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestCredentialDeleteBlockedWhileReferenced(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	credID := createCredID(t, h, "op",
		`{"name":"shared","kind":"db_password","values":{"username":"app","password":"v"}}`)

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
		`{"name":"api token","kind":"api_token","values":{"token":"v"}}`)

	connResp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"uses-alt","protocol":"tester","config":{"host":"h","api_credential":"`+credID+`"}}`))
	if connResp.Status != http.StatusCreated {
		t.Fatalf("create connection with alternate credential ref: want 201, got %d (%s)", connResp.Status, connResp.Body)
	}

	if resp := h.do(t, http.MethodDelete, "/api/credentials/"+credID, "op", nil); resp.Status != http.StatusConflict {
		t.Fatalf("delete while referenced through alternate field: want 409, got %d (%s)", resp.Status, resp.Body)
	}
}
