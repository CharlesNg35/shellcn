package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
)

type fakeMailer struct {
	enabled bool
	sent    int
}

func (f *fakeMailer) Enabled() bool { return f.enabled }
func (f *fakeMailer) Send(string, string, string) error {
	f.sent++
	return nil
}

func newInvitationService(enabled bool) (*service.InvitationService, *store.Store, *fakeMailer) {
	st := store.NewMemory()
	users := service.NewUserService(st.Users)
	mailer := &fakeMailer{enabled: enabled}
	return service.NewInvitationService(st.Invitations, users, mailer), st, mailer
}

func TestInvitationCreateAndAccept(t *testing.T) {
	ctx := context.Background()
	inv, st, mailer := newInvitationService(true)

	_, token, sent, err := inv.Create(ctx, "new@example.com", models.RoleOperator, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !sent {
		t.Fatal("email should be reported sent when mailer succeeds")
	}
	if mailer.sent != 1 {
		t.Errorf("expected one invite email, sent=%d", mailer.sent)
	}
	if _, err := inv.Lookup(ctx, token); err != nil {
		t.Fatalf("lookup: %v", err)
	}

	user, err := inv.Accept(ctx, token, "newuser", "s3cret-pw")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if user.Email != "new@example.com" || !user.HasRole(models.RoleOperator) {
		t.Errorf("accepted user wrong: %+v", user)
	}
	stored, err := st.Users.GetByUsername(ctx, "newuser")
	if err != nil || stored.ID != user.ID {
		t.Errorf("user not persisted: %v", err)
	}

	// The token is single-use: a second accept is rejected.
	if _, err := inv.Accept(ctx, token, "again", "pw"); !errors.Is(err, service.ErrInvitationInvalid) {
		t.Errorf("consumed invite reuse: want ErrInvitationInvalid, got %v", err)
	}
}

func TestInvitationAcceptDoesNotConsumeOnUserCreateFailure(t *testing.T) {
	ctx := context.Background()
	inv, st, _ := newInvitationService(false)

	if err := st.Users.Create(ctx, &models.User{ID: "existing", Username: "taken"}, "hash"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, token, _, err := inv.Create(ctx, "retry@example.com", models.RoleViewer, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if _, err := inv.Accept(ctx, token, "taken", "s3cret-pw"); !errors.Is(err, models.ErrConflict) {
		t.Fatalf("accept with taken username: want conflict, got %v", err)
	}
	user, err := inv.Accept(ctx, token, "retry", "s3cret-pw")
	if err != nil {
		t.Fatalf("accept retry should succeed: %v", err)
	}
	if user.Username != "retry" {
		t.Fatalf("retry user = %q", user.Username)
	}
}

func TestInvitationAcceptValidatesPasswordBeforeConsuming(t *testing.T) {
	ctx := context.Background()
	inv, _, _ := newInvitationService(false)

	_, token, _, err := inv.Create(ctx, "weak@example.com", models.RoleViewer, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if _, err := inv.Accept(ctx, token, "weak", "short"); err == nil {
		t.Fatal("weak password should be rejected")
	}
	if _, err := inv.Accept(ctx, token, "strong", "s3cret-pw"); err != nil {
		t.Fatalf("accept after weak password retry should succeed: %v", err)
	}
}

func TestInvitationRevoke(t *testing.T) {
	ctx := context.Background()
	inv, _, _ := newInvitationService(false)

	rec, token, _, err := inv.Create(ctx, "x@example.com", models.RoleViewer, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := inv.Revoke(ctx, rec.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := inv.Lookup(ctx, token); !errors.Is(err, service.ErrInvitationInvalid) {
		t.Errorf("revoked invite: want ErrInvitationInvalid, got %v", err)
	}
}

func TestInvitationMailerOptional(t *testing.T) {
	ctx := context.Background()
	inv, _, mailer := newInvitationService(false)

	_, token, sent, err := inv.Create(ctx, "y@example.com", models.RoleViewer, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sent {
		t.Fatal("disabled mailer should not be reported sent")
	}
	if mailer.sent != 0 {
		t.Errorf("disabled mailer should not send, sent=%d", mailer.sent)
	}
	if token == "" {
		t.Error("token (for the copyable link) must be returned even without email")
	}
}
