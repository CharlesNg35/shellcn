package service_test

import (
	"context"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/store"
)

func TestProtocolAvailabilityAllows(t *testing.T) {
	cases := []struct {
		state          models.ProtocolAvailability
		admin, regular bool
	}{
		{"", true, true},                        // unset defaults to enabled
		{models.ProtocolEnabled, true, true},    //
		{models.ProtocolAdminOnly, true, false}, // admins only
		{models.ProtocolDisabled, false, false}, // nobody
	}
	for _, c := range cases {
		if got := c.state.Allows(true); got != c.admin {
			t.Errorf("%q admin: got %v want %v", c.state, got, c.admin)
		}
		if got := c.state.Allows(false); got != c.regular {
			t.Errorf("%q regular: got %v want %v", c.state, got, c.regular)
		}
	}
}

func TestProtocolServiceSetAndAllowed(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	svc := service.NewProtocolService(st.ProtocolSettings)

	// An unset protocol is available to everyone.
	if ok, err := svc.Allowed(ctx, "ssh", false); err != nil || !ok {
		t.Fatalf("default allowed: ok=%v err=%v", ok, err)
	}

	if err := svc.Set(ctx, "ssh", models.ProtocolAdminOnly); err != nil {
		t.Fatalf("set admin_only: %v", err)
	}
	if ok, _ := svc.Allowed(ctx, "ssh", false); ok {
		t.Error("admin_only should deny a non-admin")
	}
	if ok, _ := svc.Allowed(ctx, "ssh", true); !ok {
		t.Error("admin_only should allow an admin")
	}

	if err := svc.Set(ctx, "ssh", models.ProtocolDisabled); err != nil {
		t.Fatalf("set disabled: %v", err)
	}
	if ok, _ := svc.Allowed(ctx, "ssh", true); ok {
		t.Error("disabled should deny even an admin")
	}

	states, err := svc.States(ctx)
	if err != nil {
		t.Fatalf("states: %v", err)
	}
	if states["ssh"] != models.ProtocolDisabled {
		t.Errorf("states[ssh]=%q want disabled", states["ssh"])
	}
}

func TestProtocolServiceSetRejectsUnknown(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	svc := service.NewProtocolService(st.ProtocolSettings)

	if err := svc.Set(ctx, "ssh", models.ProtocolAvailability("bogus")); err == nil {
		t.Fatal("expected an error for an unknown availability state")
	}
	if err := svc.Set(ctx, "", models.ProtocolEnabled); err == nil {
		t.Fatal("expected an error for an empty protocol")
	}
}
