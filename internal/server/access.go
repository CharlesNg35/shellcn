package server

import "github.com/charlesng35/shellcn/internal/models"

// canCreate reports whether a user may create their own resources (connections,
// credentials, folders). Viewers consume only what is shared to them; operators
// and admins create their own. Enforced on every create route — the hidden UI
// affordance is convenience only.
func canCreate(user models.User) bool {
	return user.HasRole(models.RoleOperator) || user.HasRole(models.RoleAdmin)
}
