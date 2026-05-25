package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

func createConnID(t *testing.T, resp apiResp) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil || out.ID == "" {
		t.Fatalf("no connection id in create response: %s", resp.Body)
	}
	return out.ID
}

func TestConnectionCRUDRoundTrip(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	body := `{"name":"db1","protocol":"tester","transport":"direct","config":{"host":"db.local","password":"s3cret-value"}}`
	resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(body))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "s3cret-value") {
		t.Fatalf("create response leaked secret: %s", resp.Body)
	}
	id := createConnID(t, resp)

	// Detail read: non-secret config + per-field set/not set, never the value.
	resp = h.do(t, http.MethodGet, "/api/connections/"+id, "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("detail: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	det := string(resp.Body)
	if strings.Contains(det, "s3cret-value") {
		t.Fatalf("detail leaked secret: %s", det)
	}
	if !strings.Contains(det, "db.local") || !strings.Contains(det, `"password":"set"`) {
		t.Fatalf("detail missing config/secret-state: %s", det)
	}

	conn, _ := h.store.Connections.Get(ctx, id)
	before := conn.Secrets["password"]
	if len(before) == 0 {
		t.Fatal("secret was not stored as ciphertext")
	}

	// Update omitting the secret keeps the stored ciphertext untouched.
	upd := `{"name":"db1-renamed","transport":"direct","config":{"host":"db.local"}}`
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op", strings.NewReader(upd))
	if resp.Status != http.StatusOK {
		t.Fatalf("update(keep secret): want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if conn.Name != "db1-renamed" {
		t.Fatalf("update did not persist name: %q", conn.Name)
	}
	if !bytes.Equal(conn.Secrets["password"], before) {
		t.Fatal("omitted secret should keep the stored value")
	}

	// Update with a new secret replaces the ciphertext.
	upd2 := `{"name":"db1-renamed","transport":"direct","config":{"host":"db.local","password":"rotated-value"}}`
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op", strings.NewReader(upd2))
	if resp.Status != http.StatusOK {
		t.Fatalf("update(replace secret): want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "rotated-value") {
		t.Fatalf("update response leaked secret: %s", resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if bytes.Equal(conn.Secrets["password"], before) {
		t.Fatal("replaced secret should change the ciphertext")
	}

	// Delete removes it.
	resp = h.do(t, http.MethodDelete, "/api/connections/"+id, "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("delete: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if _, err := h.store.Connections.Get(ctx, id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("connection should be gone, got err=%v", err)
	}
}

func TestConnectionCreateValidation(t *testing.T) {
	h := newHarness(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing required host", `{"name":"x","protocol":"tester","config":{}}`},
		{"unknown plugin", `{"name":"x","protocol":"ghost","config":{"host":"h"}}`},
		{"unsupported transport", `{"name":"x","protocol":"tester","transport":"satellite","config":{"host":"h"}}`},
	}
	for _, tc := range cases {
		resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(tc.body))
		if resp.Status != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d (%s)", tc.name, resp.Status, resp.Body)
		}
	}
}

func TestConnectionUpdateAuthz(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"owned","protocol":"tester","config":{"host":"h"}}`))
	id := createConnID(t, resp)

	// A stranger (viewer) without owner/manage grant is denied every mutation + the edit read.
	for _, m := range []struct {
		method string
		body   string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{"name":"hax","config":{"host":"h"}}`},
		{http.MethodDelete, ""},
	} {
		var rdr *strings.Reader
		if m.body != "" {
			rdr = strings.NewReader(m.body)
		}
		var resp apiResp
		if rdr != nil {
			resp = h.do(t, m.method, "/api/connections/"+id, "viewer", rdr)
		} else {
			resp = h.do(t, m.method, "/api/connections/"+id, "viewer", nil)
		}
		if resp.Status != http.StatusForbidden {
			t.Errorf("viewer %s: want 403, got %d", m.method, resp.Status)
		}
	}

	// Admin may manage any connection.
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "admin",
		strings.NewReader(`{"name":"admin-edit","config":{"host":"h"}}`))
	if resp.Status != http.StatusOK {
		t.Errorf("admin update: want 200, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestConnectionCredentialRefUsability(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// A credential owned by admin — op may not use it.
	_ = h.store.Credentials.Create(ctx, &models.Credential{ID: "cred-admin", Name: "adm", Kind: "db_password", OwnerID: "admin"})
	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"cred-admin"}}`))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("referencing an unusable credential: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	// A credential op owns — referencing it succeeds.
	_ = h.store.Credentials.Create(ctx, &models.Credential{ID: "cred-op", Name: "mine", Kind: "db_password", OwnerID: "op"})
	resp = h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"cred-op"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("referencing an owned credential: want 201, got %d (%s)", resp.Status, resp.Body)
	}

	_ = h.store.Credentials.Create(ctx, &models.Credential{ID: "cred-wrong-kind", Name: "wrong", Kind: "ssh_password", OwnerID: "op"})
	resp = h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"cred-wrong-kind"}}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("referencing an incompatible credential kind: want 400, got %d (%s)", resp.Status, resp.Body)
	}

	_ = h.store.Credentials.Create(ctx, &models.Credential{ID: "cred-wrong-protocol", Name: "proto", Kind: "db_password", OwnerID: "op", Protocols: []string{"other"}})
	resp = h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"x","protocol":"tester","config":{"host":"h","credential_id":"cred-wrong-protocol"}}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("referencing an incompatible credential protocol: want 400, got %d (%s)", resp.Status, resp.Body)
	}
}
