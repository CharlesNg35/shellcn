package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
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

func TestConnectionConfigVisibilityFollowsTransport(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	body := `{"name":"ctx","protocol":"tester","transport":"direct","config":{"host":"db.local","direct_secret":"direct-only","password":"shared"}}`
	resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(body))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create direct: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	conn, _ := h.store.Connections.Get(ctx, id)
	if conn.Config["host"] != "db.local" || len(conn.Secrets["direct_secret"]) == 0 {
		t.Fatalf("direct-only fields were not stored while visible: config=%v secrets=%v", conn.Config, conn.Secrets)
	}

	update := `{"name":"ctx","transport":"agent","config":{"password":"shared"}}`
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op", strings.NewReader(update))
	if resp.Status != http.StatusOK {
		t.Fatalf("switch to agent: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if _, ok := conn.Config["host"]; ok {
		t.Fatalf("hidden direct host should be removed after switching to agent: %v", conn.Config)
	}
	if _, ok := conn.Secrets["direct_secret"]; ok {
		t.Fatalf("hidden direct secret should not be preserved after switching to agent: %v", conn.Secrets)
	}
}

func TestConnectionConfigAppliesManifestDefaults(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"defaults","protocol":"tester","config":{"host":"h"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	conn, _ := h.store.Connections.Get(ctx, id)
	if conn.Config["read_only"] != true {
		t.Fatalf("default toggle not stored on create: %#v", conn.Config)
	}

	resp = h.do(t, http.MethodGet, "/api/connections/"+id, "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"read_only":true`) {
		t.Fatalf("detail missing default toggle: status=%d body=%s", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op",
		strings.NewReader(`{"name":"defaults","config":{"host":"h","read_only":false}}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("update: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if conn.Config["read_only"] != false {
		t.Fatalf("explicit false toggle was not preserved: %#v", conn.Config)
	}
}

func TestAgentOnlyConnectionDefaultsAndRejectsDirect(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"agent","protocol":"agentonly","config":{}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create agent-only: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	conn, _ := h.store.Connections.Get(ctx, id)
	if conn.Transport != "agent" {
		t.Fatalf("agent-only connection transport = %q, want agent", conn.Transport)
	}

	resp = h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"direct","protocol":"agentonly","transport":"direct","config":{}}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("create direct agent-only: want 400, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestConnectionRecordingPolicy(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Default: no recording field → recording stays off (nil/empty policy).
	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"r1","protocol":"tester","config":{"host":"h"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	conn, _ := h.store.Connections.Get(ctx, id)
	if len(conn.Recording) != 0 {
		t.Fatalf("recording must default to off, got %v", conn.Recording)
	}

	// Supported class accepts an explicit policy.
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op",
		strings.NewReader(`{"name":"r1","config":{"host":"h"},"recording":{"terminal":"auto"}}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("set terminal=auto: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if conn.Recording["terminal"] != "auto" {
		t.Fatalf("recording policy not persisted: %v", conn.Recording)
	}

	// An update that omits recording preserves the stored policy.
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op",
		strings.NewReader(`{"name":"r1-renamed","config":{"host":"h"}}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("omit recording: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if conn.Recording["terminal"] != "auto" {
		t.Fatalf("omitting recording must preserve policy, got %v", conn.Recording)
	}

	// A class the plugin does not declare is rejected.
	if r := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"r2","protocol":"internal","config":{},"recording":{"terminal":"auto"}}`)); r.Status != http.StatusBadRequest {
		t.Errorf("unsupported recording class: want 400, got %d (%s)", r.Status, r.Body)
	}
	// Invalid policy value is rejected.
	if r := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"r3","protocol":"tester","config":{"host":"h"},"recording":{"terminal":"always"}}`)); r.Status != http.StatusBadRequest {
		t.Errorf("invalid recording policy: want 400, got %d (%s)", r.Status, r.Body)
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

	// Admin has no implicit access — it cannot manage another user's connection.
	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "admin",
		strings.NewReader(`{"name":"admin-edit","config":{"host":"h"}}`))
	if resp.Status != http.StatusForbidden {
		t.Errorf("admin update of another's connection: want 403, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestDisconnectConnectionSessionClosesOnlyCurrentUserSession(t *testing.T) {
	h := newHarness(t)

	// op shares c-op with viewer so two distinct users can hold a session on it.
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/grants", "op",
		strings.NewReader(`{"subjectId":"viewer","access":"view"}`)); resp.Status != http.StatusCreated {
		t.Fatalf("share c-op: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("open op session: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/connections/c-op/x/tester.list", "viewer", nil); resp.Status != http.StatusOK {
		t.Fatalf("open viewer session: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if got := h.pluginSessions.Stats().Sessions; got != 2 {
		t.Fatalf("sessions before disconnect = %d, want 2", got)
	}

	resp := h.do(t, http.MethodDelete, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("disconnect session: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if got := h.pluginSessions.Stats().Sessions; got != 1 {
		t.Fatalf("sessions after disconnect = %d, want 1", got)
	}
}

func TestConnectionSessionKeepaliveConnectsAndReportsState(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodGet, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"state":"idle"`) {
		t.Fatalf("initial session status: status=%d body=%s", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPost, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("keepalive/connect: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if body := string(resp.Body); !strings.Contains(body, `"state":"connected"`) || !strings.Contains(body, `"channels":0`) {
		t.Fatalf("keepalive body missing connected state: %s", body)
	}
	if !strings.Contains(string(resp.Body), `"lastHealthCheck"`) {
		t.Fatalf("keepalive body missing health metadata: %s", resp.Body)
	}
	if got := h.pluginSessions.Stats().Sessions; got != 1 {
		t.Fatalf("sessions after keepalive = %d, want 1", got)
	}

	resp = h.do(t, http.MethodGet, "/api/connections/c-op/session", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"state":"connected"`) {
		t.Fatalf("connected session status: status=%d body=%s", resp.Status, resp.Body)
	}
}

func TestConnectionSessionKeepaliveReportsConnectFailureState(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections/c-boom/session", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("connect failure should be returned as session state: status=%d body=%s", resp.Status, resp.Body)
	}
	body := string(resp.Body)
	if !strings.Contains(body, `"state":"error"`) || !strings.Contains(body, `"reason":"unavailable"`) {
		t.Fatalf("connect failure body missing error state: %s", body)
	}

	resp = h.do(t, http.MethodGet, "/api/connections/c-boom/session", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"state":"error"`) {
		t.Fatalf("failure status should be queryable: status=%d body=%s", resp.Status, resp.Body)
	}
}

func TestConnectionSessionKeepaliveHonorsAccess(t *testing.T) {
	h := newHarness(t)

	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/session", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("viewer without grant: want 403, got %d (%s)", resp.Status, resp.Body)
	}
	if err := h.store.Grants.Create(context.Background(), &models.Grant{
		ID: "g-use-session-c-op", ConnectionID: "c-op", SubjectID: "viewer", Access: models.AccessView,
	}); err != nil {
		t.Fatalf("create grant: %v", err)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/session", "viewer", nil); resp.Status != http.StatusOK {
		t.Fatalf("viewer with grant: want 200, got %d (%s)", resp.Status, resp.Body)
	}

	if err := h.store.Users.Create(context.Background(), &models.User{ID: "norole", Username: "norole"}, ""); err != nil {
		t.Fatalf("create no-role user: %v", err)
	}
	h.sessions["norole"] = h.sessionMgr.Create("norole")
	if err := h.store.Grants.Create(context.Background(), &models.Grant{
		ID: "g-use-session-c-op-norole", ConnectionID: "c-op", SubjectID: "norole", Access: models.AccessView,
	}); err != nil {
		t.Fatalf("create no-role grant: %v", err)
	}
	if resp := h.do(t, http.MethodPost, "/api/connections/c-op/session", "norole", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("no-role grantee: want 403, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestDisconnectConnectionSessionHonorsConnectionAccess(t *testing.T) {
	h := newHarness(t)

	if resp := h.do(t, http.MethodDelete, "/api/connections/c-op/session", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("viewer without grant: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	if err := h.store.Grants.Create(context.Background(), &models.Grant{
		ID: "g-use-c-op", ConnectionID: "c-op", SubjectID: "viewer", Access: models.AccessView,
	}); err != nil {
		t.Fatalf("create grant: %v", err)
	}
	if resp := h.do(t, http.MethodDelete, "/api/connections/c-op/session", "viewer", nil); resp.Status != http.StatusOK {
		t.Fatalf("viewer with grant: want 200, got %d (%s)", resp.Status, resp.Body)
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

func TestSharedGranteeCannotReadOrModifyConnectionCredentials(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	_ = h.store.Credentials.Create(ctx, &models.Credential{
		ID: "cred-owner", Name: "owner-db", Kind: "db_password", OwnerID: "op",
	})
	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"shared","protocol":"tester","config":{"host":"h","credential_id":"cred-owner"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create connection: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	_ = h.store.Grants.Create(ctx, &models.Grant{
		ID: "g-manage", ConnectionID: id, SubjectID: "viewer", Access: models.AccessManage,
	})

	resp = h.do(t, http.MethodGet, "/api/connections/"+id, "viewer", nil)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("shared grantee detail: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "viewer",
		strings.NewReader(`{"name":"hax","config":{"host":"h2"},"preserveCredentials":["credential_id"]}`))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("shared grantee preserve: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "viewer",
		strings.NewReader(`{"name":"hax","config":{"host":"h3","credential_id":"cred-viewer"}}`))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("shared grantee replace: want 403, got %d (%s)", resp.Status, resp.Body)
	}

	conn, _ := h.store.Connections.Get(ctx, id)
	if conn.Config["credential_id"] != "cred-owner" {
		t.Fatalf("shared grantee must not change credential, got %#v", conn.Config["credential_id"])
	}

	resp = h.do(t, http.MethodPut, "/api/connections/"+id, "op",
		strings.NewReader(`{"name":"kept","config":{"host":"h4"},"preserveCredentials":["credential_id"]}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("owner preserve credential: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	conn, _ = h.store.Connections.Get(ctx, id)
	if conn.Config["credential_id"] != "cred-owner" {
		t.Fatalf("owner preserve should keep credential, got %#v", conn.Config["credential_id"])
	}
}

func TestConnectionFoldersAndLayout(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connection-folders", "op",
		strings.NewReader(`{"name":"Production","color":"blue"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create folder: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	var folder struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(resp.Body, &folder); err != nil || folder.ID == "" {
		t.Fatalf("create folder response: %s", resp.Body)
	}
	if folder.Name != "Production" || folder.Color != "blue" {
		t.Fatalf("folder fields: %+v", folder)
	}

	resp = h.do(t, http.MethodPost, "/api/connection-folders", "op",
		strings.NewReader(`{"name":"Databases","color":"teal","parentId":"`+folder.ID+`"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create child folder: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	var child struct {
		ID       string `json:"id"`
		ParentID string `json:"parentId"`
	}
	if err := json.Unmarshal(resp.Body, &child); err != nil || child.ID == "" || child.ParentID != folder.ID {
		t.Fatalf("create child folder response: %s", resp.Body)
	}

	resp = h.do(t, http.MethodPut, "/api/connections/layout", "op",
		strings.NewReader(`{"folders":[{"folderId":"`+folder.ID+`","sortOrder":4},{"folderId":"`+child.ID+`","parentId":"`+folder.ID+`","sortOrder":0}],"items":[{"connectionId":"c-boom","folderId":"`+child.ID+`","sortOrder":0},{"connectionId":"c-op","folderId":"`+folder.ID+`","sortOrder":1},{"connectionId":"c-internal","sortOrder":0}]}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("save layout: want 200, got %d (%s)", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodGet, "/api/connections", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("list connections: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	body := string(resp.Body)
	if !strings.Contains(body, `"folderId":"`+child.ID+`"`) || !strings.Contains(body, `"sortOrder":1`) {
		t.Fatalf("connection list missing placement data: %s", resp.Body)
	}

	resp = h.do(t, http.MethodGet, "/api/connection-folders", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), "Production") || !strings.Contains(string(resp.Body), `"parentId":"`+folder.ID+`"`) || !strings.Contains(string(resp.Body), `"sortOrder":4`) {
		t.Fatalf("list folders: status=%d body=%s", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPut, "/api/connection-folders/"+folder.ID, "op",
		strings.NewReader(`{"name":"Prod","color":"teal"}`))
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"color":"teal"`) {
		t.Fatalf("update folder: status=%d body=%s", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodDelete, "/api/connection-folders/"+folder.ID, "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("delete folder: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	resp = h.do(t, http.MethodGet, "/api/connections", "op", nil)
	if strings.Contains(string(resp.Body), `"folderId":"`+folder.ID+`"`) {
		t.Fatalf("folder deletion should move placements up: %s", resp.Body)
	}
	resp = h.do(t, http.MethodGet, "/api/connection-folders", "op", nil)
	if strings.Contains(string(resp.Body), `"parentId":"`+folder.ID+`"`) {
		t.Fatalf("folder deletion should reparent child folders: %s", resp.Body)
	}
}

func TestConnectionLayoutRejectsInaccessibleConnection(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPut, "/api/connections/layout", "op",
		strings.NewReader(`{"items":[{"connectionId":"c-view","sortOrder":0}]}`))
	if resp.Status != http.StatusForbidden {
		t.Fatalf("layout with inaccessible connection: want 403, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestConnectionFolderValidation(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/connection-folders", "op",
		strings.NewReader(`{"name":"","color":"blue"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("empty folder name: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	resp = h.do(t, http.MethodPost, "/api/connection-folders", "op",
		strings.NewReader(`{"name":"Bad","color":"neon"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("bad folder color: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	resp = h.do(t, http.MethodPost, "/api/connection-folders", "op",
		strings.NewReader(`{"name":"Child","color":"blue","parentId":"missing"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("bad parent folder: want 400, got %d (%s)", resp.Status, resp.Body)
	}
}
