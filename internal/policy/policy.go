// Package policy enforces RBAC (role → allowed action risk) plus per-connection
// ownership/sharing grants. v1 uses embedded Casbin; OPA is a later, additive
// option. Risk levels come from route metadata — never client-supplied.
package policy

import (
	"errors"
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

// ErrForbidden is the deny-by-default authorization failure.
var ErrForbidden = errors.New("policy: forbidden")

// rbacModel maps a role (sub) to the action risks it may perform (act). "*"
// means all. Roles are checked individually, so the user's effective set is the
// union of their roles' grants.
const rbacModel = `
[request_definition]
r = sub, act

[policy_definition]
p = sub, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && (p.act == "*" || r.act == p.act)
`

// defaultPolicies seed the built-in roles. admin does everything; operator runs
// all operational risk levels; viewer is read-only (safe).
func defaultPolicies() [][]string {
	return [][]string{
		{string(models.RoleAdmin), "*"},
		{string(models.RoleOperator), string(plugin.RiskSafe)},
		{string(models.RoleOperator), string(plugin.RiskWrite)},
		{string(models.RoleOperator), string(plugin.RiskDestructive)},
		{string(models.RoleOperator), string(plugin.RiskPrivileged)},
		{string(models.RoleViewer), string(plugin.RiskSafe)},
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
	_, err := en.e.AddPolicy(string(role), string(risk))
	return err
}

// roleAllowsRisk reports whether any of the user's roles permits the risk.
func (en *Enforcer) roleAllowsRisk(roles []models.Role, risk plugin.RiskLevel) bool {
	for _, role := range roles {
		ok, err := en.e.Enforce(string(role), string(risk))
		if err == nil && ok {
			return true
		}
	}
	return false
}

// AccessInput is everything an authorization decision needs. The caller (the
// route wrapper) resolves the connection + the user's grant before calling.
type AccessInput struct {
	User models.User
	Risk plugin.RiskLevel

	// Connection context. ConnectionID == "" means a non-connection route (e.g.
	// the plugin catalog), which only needs the role/risk gate.
	ConnectionID string
	OwnerID      string
	HasGrant     bool
	GrantAccess  models.Access
}

// Authorize applies both gates (deny-by-default):
//  1. the user's role must permit the route's risk, and
//  2. for connection routes, the user must own it, hold a grant, or be admin.
func (en *Enforcer) Authorize(in AccessInput) error {
	if in.User.Disabled {
		return fmt.Errorf("%w: account disabled", ErrForbidden)
	}
	if !en.roleAllowsRisk(in.User.Roles, in.Risk) {
		return fmt.Errorf("%w: role may not perform %q actions", ErrForbidden, in.Risk)
	}
	if in.ConnectionID != "" && !en.canAccessConnection(in) {
		return fmt.Errorf("%w: no access to connection %q", ErrForbidden, in.ConnectionID)
	}
	return nil
}

func (en *Enforcer) canAccessConnection(in AccessInput) bool {
	if in.User.HasRole(models.RoleAdmin) {
		return true
	}
	if in.OwnerID != "" && in.OwnerID == in.User.ID {
		return true
	}
	return in.HasGrant
}
