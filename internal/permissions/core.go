package permissions

func init() {
	perms := []*Permission{
		{
			ID:          "user.view",
			Module:      "core",
			Description: "View users",
		},
		{
			ID:          "user.create",
			Module:      "core",
			DependsOn:   []string{"user.view"},
			Description: "Create new users",
		},
		{
			ID:          "user.edit",
			Module:      "core",
			DependsOn:   []string{"user.view"},
			Description: "Edit existing users",
		},
		{
			ID:          "user.delete",
			Module:      "core",
			DependsOn:   []string{"user.view", "user.edit"},
			Description: "Delete users",
		},
		{
			ID:          "org.view",
			Module:      "core",
			Description: "View organizations",
		},
		{
			ID:          "org.create",
			Module:      "core",
			DependsOn:   []string{"org.view"},
			Description: "Create organizations",
		},
		{
			ID:          "org.manage",
			Module:      "core",
			DependsOn:   []string{"org.view"},
			Description: "Manage organizations",
		},
		{
			ID:          "connection.view",
			Module:      "core",
			Description: "View connection protocols and resources",
		},
		{
			ID:          "connection.launch",
			Module:      "core",
			DependsOn:   []string{"connection.view"},
			Description: "Launch connections",
		},
		{
			ID:          "connection.manage",
			Module:      "core",
			DependsOn:   []string{"connection.view"},
			Description: "Create and update connections",
		},
		{
			ID:          "connection.share",
			Module:      "core",
			DependsOn:   []string{"connection.manage"},
			Description: "Manage connection sharing and visibility",
		},
		{
			ID:          "vault.view",
			Module:      "core",
			Description: "View credential vault entries",
		},
		{
			ID:          "vault.create",
			Module:      "core",
			DependsOn:   []string{"vault.view"},
			Description: "Create credential vault entries",
		},
		{
			ID:          "vault.edit",
			Module:      "core",
			DependsOn:   []string{"vault.view"},
			Description: "Edit credential vault entries",
		},
		{
			ID:          "vault.delete",
			Module:      "core",
			DependsOn:   []string{"vault.view"},
			Description: "Delete credential vault entries",
		},
		{
			ID:          "vault.share",
			Module:      "core",
			DependsOn:   []string{"vault.view", "vault.edit"},
			Description: "Share credential vault entries",
		},
		{
			ID:          "vault.use_shared",
			Module:      "core",
			DependsOn:   []string{"vault.view"},
			Description: "Use shared credential vault entries",
		},
		{
			ID:          "vault.manage_all",
			Module:      "core",
			DependsOn:   []string{"vault.view", "vault.edit", "vault.delete"},
			Description: "Manage all credential vault entries",
		},
		{
			ID:          "permission.view",
			Module:      "core",
			Description: "View permissions",
		},
		{
			ID:          "permission.manage",
			Module:      "core",
			DependsOn:   []string{"permission.view"},
			Description: "Assign and revoke permissions",
		},
		{
			ID:          "audit.view",
			Module:      "core",
			Description: "View audit logs",
		},
		{
			ID:          "audit.export",
			Module:      "core",
			DependsOn:   []string{"audit.view"},
			Description: "Export audit logs",
		},
		{
			ID:          "security.audit",
			Module:      "core",
			DependsOn:   []string{"audit.view"},
			Description: "Run security audits",
		},
		{
			ID:          "notification.view",
			Module:      "core",
			Description: "View in-app notifications",
		},
		{
			ID:          "notification.manage",
			Module:      "core",
			DependsOn:   []string{"notification.view"},
			Description: "Manage in-app notifications and broadcasts",
		},
	}

	for _, perm := range perms {
		if err := Register(perm); err != nil {
			panic(err)
		}
	}
}
