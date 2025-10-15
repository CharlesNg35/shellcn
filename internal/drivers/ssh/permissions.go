package ssh

import "github.com/charlesng35/shellcn/internal/permissions"

func init() {
	registerPermissions()
}

func registerPermissions() {
	must(permissions.RegisterProtocolPermission(DriverIDSSH, "connect", &permissions.Permission{
		DisplayName:  "SSH Connect",
		Description:  "Launch interactive SSH sessions",
		DefaultScope: "resource",
		DependsOn:    []string{"connection.launch"},
		Metadata: map[string]any{
			"capability": "terminal",
		},
	}))

	must(permissions.RegisterProtocolPermission(DriverIDSSH, "sftp", &permissions.Permission{
		DisplayName:  "SSH File Transfer",
		Description:  "Access remote files through SFTP",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:ssh.connect"},
		Metadata: map[string]any{
			"capability": "file_transfer",
		},
	}))

	must(permissions.RegisterProtocolPermission(DriverIDSSH, "share", &permissions.Permission{
		DisplayName:  "SSH Session Share",
		Description:  "Share active SSH sessions with other users",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:ssh.connect", "connection.share"},
		Metadata: map[string]any{
			"capability": "collaboration",
		},
	}))

	must(permissions.RegisterProtocolPermission(DriverIDSSH, "grant_write", &permissions.Permission{
		DisplayName:  "SSH Grant Write Access",
		Description:  "Delegate write control within a shared SSH session",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:ssh.share"},
	}))

	must(permissions.RegisterProtocolPermission(DriverIDSSH, "record", &permissions.Permission{
		DisplayName:  "SSH Session Recording",
		Description:  "Record SSH terminal sessions",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:ssh.connect", "connection.manage"},
		Metadata: map[string]any{
			"capability": "session_recording",
		},
	}))

	must(permissions.RegisterProtocolPermission(DriverIDSSH, "manage_snippets", &permissions.Permission{
		DisplayName:  "SSH Snippet Management",
		Description:  "Manage reusable command snippets for SSH sessions",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:ssh.connect"},
	}))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
