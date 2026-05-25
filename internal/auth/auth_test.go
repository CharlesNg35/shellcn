package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/store"
)

func TestPasswordHashVerifyRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored in plaintext")
	}
	ok, err := auth.VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Errorf("verify correct: ok=%v err=%v", ok, err)
	}
	ok, _ = auth.VerifyPassword(hash, "wrong password")
	if ok {
		t.Error("verify wrong password returned true")
	}
}

func TestVerifyPasswordRejectsBadHash(t *testing.T) {
	if _, err := auth.VerifyPassword("not-a-phc-string", "x"); !errors.Is(err, auth.ErrInvalidHash) {
		t.Errorf("bad hash: want ErrInvalidHash, got %v", err)
	}
}

func TestLocalAuthenticator(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	hash, _ := auth.HashPassword("s3cret")
	_ = st.Users.Create(ctx, &models.User{ID: "u1", Username: "alice", Roles: []models.Role{models.RoleAdmin}}, hash)

	a := auth.NewLocalAuthenticator(st.Users)

	user, err := a.Authenticate(ctx, "alice", "s3cret")
	if err != nil || user.ID != "u1" {
		t.Fatalf("authenticate ok: user=%+v err=%v", user, err)
	}
	if _, err := a.Authenticate(ctx, "alice", "wrong"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("wrong password: want ErrInvalidCredentials, got %v", err)
	}
	// Unknown user gives the same error (no enumeration).
	if _, err := a.Authenticate(ctx, "nobody", "x"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("unknown user: want ErrInvalidCredentials, got %v", err)
	}
}

func TestLocalAuthenticatorDisabled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	hash, _ := auth.HashPassword("pw")
	_ = st.Users.Create(ctx, &models.User{ID: "u1", Username: "bob", Disabled: true}, hash)
	a := auth.NewLocalAuthenticator(st.Users)
	if _, err := a.Authenticate(ctx, "bob", "pw"); !errors.Is(err, auth.ErrAccountDisabled) {
		t.Errorf("disabled: want ErrAccountDisabled, got %v", err)
	}
}

func TestOIDCStub(t *testing.T) {
	if _, err := (auth.OIDCAuthenticator{}).Authenticate(context.Background(), "u", "p"); !errors.Is(err, auth.ErrNotImplemented) {
		t.Errorf("oidc stub: want ErrNotImplemented, got %v", err)
	}
}

func scope() auth.TicketScope {
	return auth.TicketScope{ConnectionID: "c1", RouteID: "ssh.shell", UserID: "u1", Params: map[string]string{"path": "/a"}}
}

func TestTicketRedeemHappy(t *testing.T) {
	ts := auth.NewTicketStore(0)
	tok, exp := ts.Mint(scope())
	if !exp.After(time.Now()) {
		t.Error("expiry not in the future")
	}
	if err := ts.Redeem(tok, scope()); err != nil {
		t.Errorf("redeem valid: %v", err)
	}
}

func TestTicketSingleUse(t *testing.T) {
	ts := auth.NewTicketStore(0)
	tok, _ := ts.Mint(scope())
	if err := ts.Redeem(tok, scope()); err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	if err := ts.Redeem(tok, scope()); !errors.Is(err, auth.ErrTicketInvalid) {
		t.Errorf("replay must be rejected: got %v", err)
	}
}

func TestTicketExpiry(t *testing.T) {
	ts := auth.NewTicketStore(time.Millisecond)
	tok, _ := ts.Mint(scope())
	time.Sleep(5 * time.Millisecond)
	if err := ts.Redeem(tok, scope()); !errors.Is(err, auth.ErrTicketInvalid) {
		t.Errorf("expired ticket must be rejected: got %v", err)
	}
}

func TestTicketParamMismatchRejected(t *testing.T) {
	ts := auth.NewTicketStore(0)
	tok, _ := ts.Mint(scope())
	// Same connection + route + user, but a different resource param.
	other := scope()
	other.Params = map[string]string{"path": "/b"}
	if err := ts.Redeem(tok, other); !errors.Is(err, auth.ErrTicketInvalid) {
		t.Errorf("param-mismatch must be rejected (replay against another resource): got %v", err)
	}
}

func TestTicketRouteMismatchRejected(t *testing.T) {
	ts := auth.NewTicketStore(0)
	tok, _ := ts.Mint(scope())
	other := scope()
	other.RouteID = "ssh.sftp.read"
	if err := ts.Redeem(tok, other); !errors.Is(err, auth.ErrTicketInvalid) {
		t.Errorf("route-mismatch must be rejected: got %v", err)
	}
}

func TestCheckWSOrigin(t *testing.T) {
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+"/api/ws", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	if !auth.CheckWSOrigin(mk("https://app.example.com", "app.example.com"), nil) {
		t.Error("same-site origin should pass")
	}
	if auth.CheckWSOrigin(mk("https://evil.com", "app.example.com"), nil) {
		t.Error("cross-site origin must fail")
	}
	if auth.CheckWSOrigin(mk("", "app.example.com"), nil) {
		t.Error("missing origin must fail on the WS path")
	}
	if !auth.CheckWSOrigin(mk("https://trusted.example.com", "app.example.com"), []string{"trusted.example.com"}) {
		t.Error("allowlisted origin should pass")
	}
}

func TestSessionAndCSRF(t *testing.T) {
	m := auth.NewSessionManager(time.Hour)
	s := m.Create("u1")
	if got, ok := m.Get(s.ID); !ok || got.UserID != "u1" {
		t.Fatalf("get session: ok=%v got=%+v", ok, got)
	}

	// CSRF: a request with the matching token passes; missing/wrong fails.
	r := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	r.Header.Set(auth.CSRFHeader, s.CSRFToken)
	if !s.ValidateCSRF(r) {
		t.Error("valid CSRF token rejected")
	}
	r2 := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	if s.ValidateCSRF(r2) {
		t.Error("missing CSRF token accepted")
	}
	r3 := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	r3.Header.Set(auth.CSRFHeader, "forged")
	if s.ValidateCSRF(r3) {
		t.Error("forged CSRF token accepted")
	}

	m.Destroy(s.ID)
	if _, ok := m.Get(s.ID); ok {
		t.Error("destroyed session still retrievable")
	}
}

func TestSessionExpiry(t *testing.T) {
	m := auth.NewSessionManager(time.Millisecond)
	s := m.Create("u1")
	time.Sleep(5 * time.Millisecond)
	if _, ok := m.Get(s.ID); ok {
		t.Error("expired session should not be retrievable")
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	m := auth.NewSessionManager(time.Hour)
	s := m.Create("u1")
	w := httptest.NewRecorder()
	auth.SetSessionCookie(w, s, true)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("want 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie attributes wrong: %+v", c)
	}
}
