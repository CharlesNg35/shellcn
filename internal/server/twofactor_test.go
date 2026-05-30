package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/service"
)

func TestTwoFactorLoginFlow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	user, err := service.NewUserService(h.store.Users).Create(ctx, service.NewUserInput{
		Username: "mfauser", Password: "s3cret-pw", Roles: []models.Role{models.RoleViewer},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	h.sessions["mfauser"] = h.sessionMgr.Create(user.ID)

	// Enroll and enable 2FA through the authenticated endpoints.
	setup := h.do(t, http.MethodPost, "/api/auth/totp/setup", "mfauser", nil)
	if setup.Status != http.StatusOK {
		t.Fatalf("totp setup: %d %s", setup.Status, setup.Body)
	}
	var setupResp struct {
		Secret string `json:"secret"`
		QR     string `json:"qr"`
	}
	if err := json.Unmarshal(setup.Body, &setupResp); err != nil || setupResp.Secret == "" {
		t.Fatalf("decode setup: %v body=%s", err, setup.Body)
	}
	if !strings.HasPrefix(setupResp.QR, "data:image/png;base64,") {
		t.Errorf("expected a QR data URL, got %q", setupResp.QR)
	}

	code, _ := totp.GenerateCode(setupResp.Secret, time.Now())
	enable := h.do(t, http.MethodPost, "/api/auth/totp/enable", "mfauser", strings.NewReader(fmt.Sprintf(`{"code":%q}`, code)))
	if enable.Status != http.StatusOK || !strings.Contains(string(enable.Body), "recoveryCodes") {
		t.Fatalf("totp enable: %d %s", enable.Status, enable.Body)
	}

	// Password login now returns a challenge instead of a session.
	login := h.do(t, http.MethodPost, "/api/auth/login", "", strings.NewReader(`{"username":"mfauser","password":"s3cret-pw"}`))
	if login.Status != http.StatusOK {
		t.Fatalf("login: %d %s", login.Status, login.Body)
	}
	var lr struct {
		MFARequired bool             `json:"mfaRequired"`
		MFAToken    string           `json:"mfaToken"`
		Session     *json.RawMessage `json:"session"`
	}
	if err := json.Unmarshal(login.Body, &lr); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if !lr.MFARequired || lr.MFAToken == "" || lr.Session != nil {
		t.Fatalf("expected an MFA challenge, got %s", login.Body)
	}

	// A wrong code is rejected.
	bad := h.do(t, http.MethodPost, "/api/auth/login/mfa", "",
		strings.NewReader(fmt.Sprintf(`{"mfaToken":%q,"code":"000000"}`, lr.MFAToken)))
	if bad.Status != http.StatusUnauthorized {
		t.Errorf("bad mfa code: want 401, got %d", bad.Status)
	}

	// The correct code completes the login and returns a session.
	code2, _ := totp.GenerateCode(setupResp.Secret, time.Now())
	done := h.do(t, http.MethodPost, "/api/auth/login/mfa", "",
		strings.NewReader(fmt.Sprintf(`{"mfaToken":%q,"code":%q}`, lr.MFAToken, code2)))
	if done.Status != http.StatusOK || !strings.Contains(string(done.Body), "csrfToken") {
		t.Fatalf("mfa login: %d %s", done.Status, done.Body)
	}
}
