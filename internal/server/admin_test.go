package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
)

func TestAdminUsersAuthz(t *testing.T) {
	h := newHarness(t)

	for _, u := range []string{"viewer", "op"} {
		if resp := h.do(t, http.MethodGet, "/api/admin/users", u, nil); resp.Status != http.StatusForbidden {
			t.Errorf("%s on admin users: want 403, got %d", u, resp.Status)
		}
	}
	resp := h.do(t, http.MethodGet, "/api/admin/users", "admin", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"viewer"`) {
		t.Fatalf("admin list users: status=%d body=%s", resp.Status, resp.Body)
	}
}

func TestAdminCreateUser(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/admin/users", "admin",
		strings.NewReader(`{"username":"alice","email":"a@x.com","role":"operator","password":"s3cret-pw"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create user: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "s3cret-pw") {
		t.Fatalf("create response leaked password: %s", resp.Body)
	}

	if resp := h.do(t, http.MethodGet, "/api/admin/users", "admin", nil); !strings.Contains(string(resp.Body), `"alice"`) {
		t.Errorf("created user missing from list: %s", resp.Body)
	}

	for _, body := range []string{
		`{"username":"b","role":"superuser","password":"pw"}`, // invalid role
		`{"username":"","role":"viewer","password":"pw"}`,     // missing username
		`{"username":"c","role":"viewer"}`,                    // missing password
	} {
		if resp := h.do(t, http.MethodPost, "/api/admin/users", "admin", strings.NewReader(body)); resp.Status != http.StatusBadRequest {
			t.Errorf("invalid create %q: want 400, got %d", body, resp.Status)
		}
	}
}

func TestAdminDeleteUserRootRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_ = h.store.Users.Create(ctx, &models.User{ID: "root", Username: "root", Roles: []models.Role{models.RoleAdmin}, Protected: true}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "admin2", Username: "admin2", Roles: []models.Role{models.RoleAdmin}}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "target", Username: "target", Roles: []models.Role{models.RoleViewer}}, "")
	h.sessions["root"] = h.sessionMgr.Create("root")

	// A non-root admin may delete a non-admin.
	if resp := h.do(t, http.MethodDelete, "/api/admin/users/target", "admin", nil); resp.Status != http.StatusOK {
		t.Errorf("admin delete non-admin: want 200, got %d", resp.Status)
	}
	// …but not another admin.
	if resp := h.do(t, http.MethodDelete, "/api/admin/users/admin2", "admin", nil); resp.Status != http.StatusForbidden {
		t.Errorf("non-root delete admin: want 403, got %d", resp.Status)
	}
	// The protected root admin can never be deleted.
	if resp := h.do(t, http.MethodDelete, "/api/admin/users/root", "root", nil); resp.Status != http.StatusForbidden {
		t.Errorf("delete protected root: want 403, got %d", resp.Status)
	}
	// The root admin may delete other admins.
	if resp := h.do(t, http.MethodDelete, "/api/admin/users/admin2", "root", nil); resp.Status != http.StatusOK {
		t.Errorf("root delete admin: want 200, got %d", resp.Status)
	}
}

func TestInvitationFlow(t *testing.T) {
	h := newHarness(t)

	// A viewer cannot invite.
	if resp := h.do(t, http.MethodPost, "/api/admin/invitations", "viewer",
		strings.NewReader(`{"email":"x@y.com","role":"viewer"}`)); resp.Status != http.StatusForbidden {
		t.Errorf("viewer invite: want 403, got %d", resp.Status)
	}

	resp := h.do(t, http.MethodPost, "/api/admin/invitations", "admin",
		strings.NewReader(`{"email":"new@example.com","role":"viewer"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create invite: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	var created struct {
		Link string `json:"link"`
	}
	_ = json.Unmarshal(resp.Body, &created)
	token := created.Link[strings.LastIndex(created.Link, "/invite/")+len("/invite/"):]
	if token == "" {
		t.Fatalf("no token in invite link: %s", resp.Body)
	}

	// Listed as pending for admins.
	if resp := h.do(t, http.MethodGet, "/api/admin/invitations", "admin", nil); !strings.Contains(string(resp.Body), "new@example.com") {
		t.Errorf("invite missing from list: %s", resp.Body)
	}

	// Public lookup resolves the email; accept creates the account.
	if resp := h.do(t, http.MethodGet, "/api/invitations/"+token, "", nil); resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), "new@example.com") {
		t.Fatalf("public lookup: status=%d body=%s", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodPost, "/api/invitations/"+token+"/accept", "",
		strings.NewReader(`{"username":"newbie","password":"s3cret-pw"}`)); resp.Status != http.StatusCreated {
		t.Fatalf("accept: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	if _, err := h.store.Users.GetByUsername(context.Background(), "newbie"); err != nil {
		t.Errorf("accepted user not created: %v", err)
	}

	// The token is single-use.
	if resp := h.do(t, http.MethodPost, "/api/invitations/"+token+"/accept", "",
		strings.NewReader(`{"username":"again","password":"pw"}`)); resp.Status != http.StatusNotFound {
		t.Errorf("reused invite: want 404, got %d", resp.Status)
	}
}
