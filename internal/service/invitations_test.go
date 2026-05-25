package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/store"
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

	_, token, err := inv.Create(ctx, "new@example.com", models.RoleOperator, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create: %v", err)
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

func TestInvitationRevoke(t *testing.T) {
	ctx := context.Background()
	inv, _, _ := newInvitationService(false)

	rec, token, err := inv.Create(ctx, "x@example.com", models.RoleViewer, "admin", "https://host/invite/")
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

	_, token, err := inv.Create(ctx, "y@example.com", models.RoleViewer, "admin", "https://host/invite/")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if mailer.sent != 0 {
		t.Errorf("disabled mailer should not send, sent=%d", mailer.sent)
	}
	if token == "" {
		t.Error("token (for the copyable link) must be returned even without email")
	}
}
