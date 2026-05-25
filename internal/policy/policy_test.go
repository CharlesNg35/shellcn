package policy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
	"github.com/charlesng/shellcn/internal/store"
)

func newEnforcer(t *testing.T) *policy.Enforcer {
	t.Helper()
	e, err := policy.New()
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	return e
}

func user(id string, roles ...models.Role) models.User {
	return models.User{ID: id, Roles: roles}
}

// TestRiskMatrix is the core gate: which roles may perform which risk levels,
// for a connection they own (so the access gate always passes here).
func TestRiskMatrix(t *testing.T) {
	en := newEnforcer(t)
	risks := []plugin.RiskLevel{plugin.RiskSafe, plugin.RiskWrite, plugin.RiskDestructive, plugin.RiskPrivileged}

	cases := []struct {
		role  models.Role
		allow map[plugin.RiskLevel]bool
	}{
		{models.RoleViewer, map[plugin.RiskLevel]bool{plugin.RiskSafe: true}},
		{models.RoleOperator, map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true, plugin.RiskDestructive: true, plugin.RiskPrivileged: true}},
		{models.RoleAdmin, map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true, plugin.RiskDestructive: true, plugin.RiskPrivileged: true}},
	}

	for _, c := range cases {
		for _, risk := range risks {
			in := policy.AccessInput{
				User: user("owner", c.role), Risk: risk,
				ConnectionID: "c1", OwnerID: "owner",
			}
			err := en.Authorize(in)
			allowed := err == nil
			if allowed != c.allow[risk] {
				t.Errorf("role=%s risk=%s: allowed=%v want=%v (err=%v)", c.role, risk, allowed, c.allow[risk], err)
			}
		}
	}
}

func TestViewerBlockedFromDestructiveEvenAsOwner(t *testing.T) {
	en := newEnforcer(t)
	in := policy.AccessInput{
		User: user("owner", models.RoleViewer), Risk: plugin.RiskDestructive,
		ConnectionID: "c1", OwnerID: "owner",
	}
	if err := en.Authorize(in); !errors.Is(err, policy.ErrForbidden) {
		t.Errorf("viewer must be blocked from destructive even when owner: got %v", err)
	}
}

func TestConnectionAccessGate(t *testing.T) {
	en := newEnforcer(t)
	base := policy.AccessInput{Risk: plugin.RiskWrite, ConnectionID: "c1", OwnerID: "owner"}

	// Operator who is neither owner nor grantee: denied (deny-by-default).
	stranger := base
	stranger.User = user("stranger", models.RoleOperator)
	if err := en.Authorize(stranger); !errors.Is(err, policy.ErrForbidden) {
		t.Errorf("stranger with no grant must be denied: got %v", err)
	}

	// Same operator, now with a grant: allowed.
	granted := stranger
	granted.HasGrant = true
	granted.GrantAccess = models.AccessUse
	if err := en.Authorize(granted); err != nil {
		t.Errorf("granted operator should be allowed: %v", err)
	}

	// Owner: allowed.
	owner := base
	owner.User = user("owner", models.RoleOperator)
	if err := en.Authorize(owner); err != nil {
		t.Errorf("owner should be allowed: %v", err)
	}

	// Admin: allowed on any connection (no ownership/grant needed).
	admin := base
	admin.User = user("root", models.RoleAdmin)
	if err := en.Authorize(admin); err != nil {
		t.Errorf("admin should be allowed on any connection: %v", err)
	}
}

func TestNonConnectionRouteSkipsAccessGate(t *testing.T) {
	en := newEnforcer(t)
	// A safe, non-connection route (e.g. plugin catalog) for a viewer.
	in := policy.AccessInput{User: user("v", models.RoleViewer), Risk: plugin.RiskSafe}
	if err := en.Authorize(in); err != nil {
		t.Errorf("viewer should reach a safe non-connection route: %v", err)
	}
}

func TestDisabledUserDenied(t *testing.T) {
	en := newEnforcer(t)
	u := user("u", models.RoleAdmin)
	u.Disabled = true
	in := policy.AccessInput{User: u, Risk: plugin.RiskSafe, ConnectionID: "c1", OwnerID: "u"}
	if err := en.Authorize(in); !errors.Is(err, policy.ErrForbidden) {
		t.Errorf("disabled admin must be denied: got %v", err)
	}
}

func TestNoRolesDenied(t *testing.T) {
	en := newEnforcer(t)
	in := policy.AccessInput{User: user("u"), Risk: plugin.RiskSafe}
	if err := en.Authorize(in); !errors.Is(err, policy.ErrForbidden) {
		t.Errorf("user with no roles must be denied (deny-by-default): got %v", err)
	}
}

func TestAddRolePolicy(t *testing.T) {
	en := newEnforcer(t)
	const custom models.Role = "auditor"
	in := policy.AccessInput{User: user("u", custom), Risk: plugin.RiskSafe}
	if err := en.Authorize(in); !errors.Is(err, policy.ErrForbidden) {
		t.Fatalf("custom role should start with no grants: got %v", err)
	}
	if err := en.AddRolePolicy(custom, plugin.RiskSafe); err != nil {
		t.Fatalf("add policy: %v", err)
	}
	if err := en.Authorize(in); err != nil {
		t.Errorf("custom role should now allow safe: %v", err)
	}
}

func TestPermissionSpecificPolicy(t *testing.T) {
	en := newEnforcer(t)
	const custom models.Role = "file-uploader"
	if err := en.AddRolePermissionPolicy(custom, "file.upload", plugin.RiskWrite); err != nil {
		t.Fatalf("add policy: %v", err)
	}

	allowed := policy.AccessInput{
		User:       user("u", custom),
		Permission: "file.upload",
		Risk:       plugin.RiskWrite,
	}
	if err := en.Authorize(allowed); err != nil {
		t.Fatalf("permission-specific policy should allow matching route: %v", err)
	}

	denied := allowed
	denied.Permission = "file.delete"
	if err := en.Authorize(denied); !errors.Is(err, policy.ErrForbidden) {
		t.Fatalf("different permission should be denied: %v", err)
	}
}

func TestLoadStorePolicies(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	if err := st.Policies.Create(ctx, &models.PolicyRule{ID: "p1", Role: "auditor", Permission: "audit.read", Risk: string(plugin.RiskSafe)}); err != nil {
		t.Fatalf("create policy: %v", err)
	}
	en := newEnforcer(t)
	if err := en.LoadStorePolicies(ctx, st.Policies); err != nil {
		t.Fatalf("load policies: %v", err)
	}
	in := policy.AccessInput{User: user("u", "auditor"), Permission: "audit.read", Risk: plugin.RiskSafe}
	if err := en.Authorize(in); err != nil {
		t.Fatalf("stored policy should authorize matching route: %v", err)
	}
}
