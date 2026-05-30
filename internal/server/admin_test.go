package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
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

func TestAdminDeactivateUserRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_ = h.store.Users.Create(ctx, &models.User{ID: "root", Username: "root", Roles: []models.Role{models.RoleAdmin}, Protected: true}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "admin2", Username: "admin2", Roles: []models.Role{models.RoleAdmin}}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "target", Username: "target", Roles: []models.Role{models.RoleViewer}}, "")
	h.sessions["root"] = h.sessionMgr.Create("root")

	deactivate := func(id, as string) int {
		return h.do(t, http.MethodPost, "/api/admin/users/"+id+"/deactivate", as, nil).Status
	}

	// A non-root admin may deactivate a non-admin.
	if s := deactivate("target", "admin"); s != http.StatusOK {
		t.Errorf("admin deactivate non-admin: want 200, got %d", s)
	}
	// …but not another admin, and never itself.
	if s := deactivate("admin2", "admin"); s != http.StatusForbidden {
		t.Errorf("non-root deactivate admin: want 403, got %d", s)
	}
	if s := deactivate("admin", "admin"); s != http.StatusForbidden {
		t.Errorf("self-deactivate: want 403, got %d", s)
	}
	// The protected root can never be deactivated.
	if s := deactivate("root", "root"); s != http.StatusForbidden {
		t.Errorf("deactivate protected root: want 403, got %d", s)
	}
	// A non-root admin still cannot edit another admin via update.
	if resp := h.do(t, http.MethodPut, "/api/admin/users/admin2", "admin",
		strings.NewReader(`{"role":"viewer","disabled":false}`)); resp.Status != http.StatusForbidden {
		t.Errorf("non-root demote admin: want 403, got %d", resp.Status)
	}
	// The root admin may deactivate another admin and re-activate it.
	if s := deactivate("admin2", "root"); s != http.StatusOK {
		t.Errorf("root deactivate admin: want 200, got %d", s)
	}
	if s := h.do(t, http.MethodPost, "/api/admin/users/admin2/activate", "root", nil).Status; s != http.StatusOK {
		t.Errorf("root activate admin: want 200, got %d", s)
	}
}

func TestAdminUpdateUserRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_ = h.store.Users.Create(ctx, &models.User{ID: "root", Username: "root", Roles: []models.Role{models.RoleAdmin}, Protected: true}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "target", Username: "target", Roles: []models.Role{models.RoleViewer}}, "")
	h.sessions["root"] = h.sessionMgr.Create("root")

	update := func(id, as, body string) int {
		return h.do(t, http.MethodPut, "/api/admin/users/"+id, as, strings.NewReader(body)).Status
	}

	// No one edits their own account from the admin list — not a regular admin…
	if s := update("admin", "admin", `{"role":"admin"}`); s != http.StatusForbidden {
		t.Errorf("admin self-edit: want 403, got %d", s)
	}
	// …and not the root admin (it uses its profile instead).
	if s := update("root", "root", `{"role":"admin"}`); s != http.StatusForbidden {
		t.Errorf("root self-edit: want 403, got %d", s)
	}
	// The root admin is immutable from here even for another admin.
	if s := update("root", "admin", `{"role":"viewer"}`); s != http.StatusForbidden {
		t.Errorf("edit protected root: want 403, got %d", s)
	}
	// A regular admin may still edit a non-admin user.
	if s := update("target", "admin", `{"role":"operator"}`); s != http.StatusOK {
		t.Errorf("admin edit non-admin: want 200, got %d", s)
	}
}

func TestAdminResetTwoFactorRules(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_ = h.store.Users.Create(ctx, &models.User{ID: "root", Username: "root", Roles: []models.Role{models.RoleAdmin}, Protected: true}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "admin2", Username: "admin2", Roles: []models.Role{models.RoleAdmin}}, "")
	_ = h.store.Users.Create(ctx, &models.User{ID: "target", Username: "target", Roles: []models.Role{models.RoleViewer}}, "")
	_ = h.store.Users.SetTwoFactor(ctx, "target", []byte("secret"), true, []string{"hash"})
	h.sessions["root"] = h.sessionMgr.Create("root")

	reset := func(id, as string) int {
		return h.do(t, http.MethodPost, "/api/admin/users/"+id+"/reset-2fa", as, nil).Status
	}

	if s := reset("admin", "admin"); s != http.StatusForbidden {
		t.Errorf("self reset: want 403, got %d", s)
	}
	if s := reset("root", "root"); s != http.StatusForbidden {
		t.Errorf("reset protected root: want 403, got %d", s)
	}
	if s := reset("admin2", "admin"); s != http.StatusForbidden {
		t.Errorf("non-root reset admin: want 403, got %d", s)
	}
	// A regular admin resets a non-admin, which clears the 2FA state.
	if s := reset("target", "admin"); s != http.StatusOK {
		t.Fatalf("admin reset non-admin: want 200, got %d", s)
	}
	if u, _ := h.store.Users.GetByID(ctx, "target"); u.TOTPEnabled || len(u.TOTPSecret) != 0 {
		t.Errorf("reset did not clear 2FA: %+v", u)
	}
	// The root admin may reset another admin.
	_ = h.store.Users.SetTwoFactor(ctx, "admin2", []byte("s"), true, nil)
	if s := reset("admin2", "root"); s != http.StatusOK {
		t.Errorf("root reset admin: want 200, got %d", s)
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
