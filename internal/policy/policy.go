// Package policy enforces RBAC (role → allowed permission/risk) plus per-connection
// ownership/sharing grants, using embedded Casbin. Risk levels come from route
// metadata — never client-supplied.
package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ErrForbidden is the deny-by-default authorization failure.
var ErrForbidden = errors.New("policy: forbidden")

// rbacModel maps a role (sub) to route permission (obj) + risk (act). "*"
// means all. Roles are checked individually, so the user's effective set is the
// union of their roles' grants.
const rbacModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && (p.obj == "*" || p.obj == r.obj) && (p.act == "*" || p.act == r.act)
`

// defaultPolicies seed the built-in roles. admin does everything; operator runs
// all operational risk levels; viewer is read-only (safe).
func defaultPolicies() [][]string {
	return [][]string{
		{string(models.RoleAdmin), "*", "*"},
		{string(models.RoleOperator), "*", string(plugin.RiskSafe)},
		{string(models.RoleOperator), "*", string(plugin.RiskWrite)},
		{string(models.RoleOperator), "*", string(plugin.RiskDestructive)},
		{string(models.RoleOperator), "*", string(plugin.RiskPrivileged)},
		{string(models.RoleViewer), "*", string(plugin.RiskSafe)},
	}
}

// Enforcer answers authorization decisions. It is safe for concurrent use.
type Enforcer struct {
	e *casbin.Enforcer
}

// New builds an enforcer seeded with the default role policies.
func New() (*Enforcer, error) {
	m, err := model.NewModelFromString(rbacModel)
	if err != nil {
		return nil, fmt.Errorf("policy model: %w", err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("policy enforcer: %w", err)
	}
	if _, err := e.AddPolicies(defaultPolicies()); err != nil {
		return nil, fmt.Errorf("seed policies: %w", err)
	}
	return &Enforcer{e: e}, nil
}

// AddRolePolicy grants a role an additional risk level (extensibility hook).
func (en *Enforcer) AddRolePolicy(role models.Role, risk plugin.RiskLevel) error {
	return en.AddRolePermissionPolicy(role, "*", risk)
}

// AddRolePermissionPolicy grants a role a route permission/risk pair.
func (en *Enforcer) AddRolePermissionPolicy(role models.Role, permission string, risk plugin.RiskLevel) error {
	if permission == "" {
		permission = "*"
	}
	_, err := en.e.AddPolicy(string(role), permission, string(risk))
	return err
}

// LoadStorePolicies loads additive policy rows from the control-plane store.
func (en *Enforcer) LoadStorePolicies(ctx context.Context, policies store.PolicyStore) error {
	if policies == nil {
		return nil
	}
	rows, err := policies.List(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := en.AddRolePermissionPolicy(row.Role, row.Permission, plugin.RiskLevel(row.Risk)); err != nil {
			return fmt.Errorf("load policy %q: %w", row.ID, err)
		}
	}
	return nil
}

// roleAllows reports whether any of the user's roles permits the permission/risk.
func (en *Enforcer) roleAllows(roles []models.Role, permission string, risk plugin.RiskLevel) bool {
	if permission == "" {
		permission = "*"
	}
	for _, role := range roles {
		ok, err := en.e.Enforce(string(role), permission, string(risk))
		if err == nil && ok {
			return true
		}
	}
	return false
}

// AccessInput is everything an authorization decision needs. The caller (the
// route wrapper) resolves the connection + the user's grant before calling.
type AccessInput struct {
	User       models.User
	Permission string
	Risk       plugin.RiskLevel

	// Connection context. ConnectionID == "" means a non-connection route (e.g.
	// the plugin catalog), which only needs the role/risk gate.
	ConnectionID string
	OwnerID      string
	HasGrant     bool
	GrantAccess  models.Access
}

// Authorize applies the route gate and, for shared connections, the grant tier.
// Owners are constrained by their role. Grantees are constrained by the grant
// they received, while still needing an active account with at least one role.
//
// Admin is a user-management role, not a super-user: it grants no implicit access
// to other users' connections.
func (en *Enforcer) Authorize(in AccessInput) error {
	if in.User.Disabled {
		return fmt.Errorf("%w: account disabled", ErrForbidden)
	}
	if !hasAnyRole(in.User.Roles) {
		return fmt.Errorf("%w: role may not perform %q/%q actions", ErrForbidden, in.Permission, in.Risk)
	}
	if in.ConnectionID == "" {
		if !en.roleAllows(in.User.Roles, in.Permission, in.Risk) {
			return fmt.Errorf("%w: role may not perform %q/%q actions", ErrForbidden, in.Permission, in.Risk)
		}
		return nil
	}
	if in.OwnerID != "" && in.OwnerID == in.User.ID {
		if !en.roleAllows(in.User.Roles, in.Permission, in.Risk) {
			return fmt.Errorf("%w: role may not perform %q/%q actions", ErrForbidden, in.Permission, in.Risk)
		}
		return nil
	}
	if !in.HasGrant {
		return fmt.Errorf("%w: no access to connection %q", ErrForbidden, in.ConnectionID)
	}
	if !grantAllows(in.GrantAccess, in.Risk) {
		return fmt.Errorf("%w: grant %q may not perform %q/%q actions", ErrForbidden, in.GrantAccess, in.Permission, in.Risk)
	}
	return nil
}

func hasAnyRole(roles []models.Role) bool {
	return len(roles) > 0
}

var grantRiskLevel = map[models.Access]int{
	models.AccessView:       1,
	models.AccessManage:     3,
	models.AccessPrivileged: 4,
}

var routeRiskLevel = map[plugin.RiskLevel]int{
	plugin.RiskSafe:        1,
	plugin.RiskWrite:       2,
	plugin.RiskDestructive: 3,
	plugin.RiskPrivileged:  4,
}

func grantAllows(access models.Access, risk plugin.RiskLevel) bool {
	grantCeiling, ok := grantRiskLevel[access]
	if !ok {
		return false
	}
	level, ok := routeRiskLevel[risk]
	if !ok {
		return false
	}
	return level <= grantCeiling
}
