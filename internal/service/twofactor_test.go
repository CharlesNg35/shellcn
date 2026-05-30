package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/secrets"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
)

func newTwoFactor(t *testing.T) (*service.TwoFactorService, *store.Store) {
	t.Helper()
	st := store.NewMemory()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	vault, err := secrets.NewVault(key)
	if err != nil {
		t.Fatalf("vault: %v", err)
	}
	return service.NewTwoFactorService(st.Users, vault, "ShellCN"), st
}

func enroll(t *testing.T, tf *service.TwoFactorService, st *store.Store, userID string) string {
	t.Helper()
	ctx := context.Background()
	user, err := st.Users.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	en, err := tf.BeginEnrollment(ctx, user)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	return en.Secret
}

func TestTwoFactorEnrollVerifyDisable(t *testing.T) {
	ctx := context.Background()
	tf, st := newTwoFactor(t)
	_ = st.Users.Create(ctx, &models.User{ID: "u1", Username: "alice", Roles: []models.Role{models.RoleViewer}}, "")

	secret := enroll(t, tf, st, "u1")

	// A wrong code does not enable 2FA.
	user, _ := st.Users.GetByID(ctx, "u1")
	if _, err := tf.ConfirmEnrollment(ctx, user, "000000"); err == nil {
		t.Fatal("confirm with bad code should fail")
	}
	if u, _ := st.Users.GetByID(ctx, "u1"); u.TOTPEnabled {
		t.Fatal("2FA should not be enabled after a bad confirm")
	}

	// Confirming with a valid code enables 2FA and returns recovery codes.
	code, _ := totp.GenerateCode(secret, time.Now())
	user, _ = st.Users.GetByID(ctx, "u1")
	recovery, err := tf.ConfirmEnrollment(ctx, user, code)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if len(recovery) == 0 {
		t.Fatal("expected recovery codes")
	}
	user, _ = st.Users.GetByID(ctx, "u1")
	if !user.TOTPEnabled {
		t.Fatal("2FA should be enabled")
	}

	// A current TOTP code verifies.
	code, _ = totp.GenerateCode(secret, time.Now())
	if ok, err := tf.Verify(ctx, user, code); err != nil || !ok {
		t.Fatalf("verify totp: ok=%v err=%v", ok, err)
	}

	// A recovery code verifies once, then is consumed.
	user, _ = st.Users.GetByID(ctx, "u1")
	if ok, err := tf.Verify(ctx, user, recovery[0]); err != nil || !ok {
		t.Fatalf("verify recovery: ok=%v err=%v", ok, err)
	}
	user, _ = st.Users.GetByID(ctx, "u1")
	if ok, _ := tf.Verify(ctx, user, recovery[0]); ok {
		t.Fatal("recovery code should be single-use")
	}

	// Disable requires a valid code and clears the secret.
	code, _ = totp.GenerateCode(secret, time.Now())
	user, _ = st.Users.GetByID(ctx, "u1")
	if err := tf.Disable(ctx, user, code); err != nil {
		t.Fatalf("disable: %v", err)
	}
	user, _ = st.Users.GetByID(ctx, "u1")
	if user.TOTPEnabled || len(user.TOTPSecret) != 0 {
		t.Fatal("disable should clear 2FA state")
	}
}

func TestTwoFactorReEnrollRejected(t *testing.T) {
	ctx := context.Background()
	tf, st := newTwoFactor(t)
	_ = st.Users.Create(ctx, &models.User{ID: "u1", Username: "alice", Roles: []models.Role{models.RoleViewer}}, "")

	secret := enroll(t, tf, st, "u1")
	code, _ := totp.GenerateCode(secret, time.Now())
	user, _ := st.Users.GetByID(ctx, "u1")
	if _, err := tf.ConfirmEnrollment(ctx, user, code); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// Re-enrolling over active 2FA must be rejected and leave the secret and
	// recovery codes untouched (no silent drop of protection).
	user, _ = st.Users.GetByID(ctx, "u1")
	if _, err := tf.BeginEnrollment(ctx, user); !errors.Is(err, service.ErrTOTPAlreadyEnabled) {
		t.Fatalf("re-enroll: want ErrTOTPAlreadyEnabled, got %v", err)
	}
	user, _ = st.Users.GetByID(ctx, "u1")
	if !user.TOTPEnabled || len(user.RecoveryCodeHashes) == 0 || len(user.TOTPSecret) == 0 {
		t.Fatal("existing 2FA must remain intact after a rejected re-enroll")
	}
}

func TestTwoFactorShouldRemind(t *testing.T) {
	tf, _ := newTwoFactor(t)
	// Never reminded → remind.
	if !tf.ShouldRemind(models.User{}) {
		t.Error("should remind a user never nudged")
	}
	// Reminded recently → don't.
	recent := time.Now().Add(-time.Hour)
	if tf.ShouldRemind(models.User{MFARemindedAt: &recent}) {
		t.Error("should not remind right after a nudge")
	}
	// Reminded long ago → remind again.
	old := time.Now().Add(-100 * time.Hour)
	if !tf.ShouldRemind(models.User{MFARemindedAt: &old}) {
		t.Error("should remind again after the interval")
	}
	// Enabled → never.
	if tf.ShouldRemind(models.User{TOTPEnabled: true}) {
		t.Error("should not remind when 2FA is enabled")
	}
}
