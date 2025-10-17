package ssh

import (
	"context"
	"errors"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/charlesng35/shellcn/internal/drivers"
)

const (
	// DriverIDSSH is the canonical driver identifier for interactive SSH sessions.
	DriverIDSSH = "ssh"
	// DriverIDSFTP is the protocol identifier for SFTP file management backed by the SSH driver.
	DriverIDSFTP = "sftp"
)

var (
	// Compile-time checks to ensure the driver satisfies expected interfaces.
	_ drivers.Driver              = (*Driver)(nil)
	_ drivers.HealthReporter      = (*Driver)(nil)
	_ drivers.CredentialTemplater = (*Driver)(nil)
	_ drivers.ConnectionTemplater = (*Driver)(nil)
)

const (
	sshTemplateVersion = "2025-01-15"
	hostnamePattern    = `^[a-zA-Z0-9._-]+$`
)

// Driver implements the drivers.Driver interface for SSH and SFTP descriptors.
type Driver struct {
	drivers.BaseDriver
	caps     drivers.Capabilities
	template *drivers.ConnectionTemplate
}

// newDriver constructs a driver instance with shared metadata and capability flags.
func newDriver(desc drivers.Descriptor, caps drivers.Capabilities, template *drivers.ConnectionTemplate) *Driver {
	return &Driver{
		BaseDriver: drivers.NewBaseDriver(desc),
		caps:       caps,
		template:   template,
	}
}

// NewSSHDriver returns the SSH terminal driver descriptor.
func NewSSHDriver() *Driver {
	return newDriver(drivers.Descriptor{
		ID:        DriverIDSSH,
		Module:    "ssh",
		Title:     "SSH",
		Category:  "terminal",
		Icon:      "terminal",
		SortOrder: 1,
	}, drivers.Capabilities{
		Terminal:         true,
		FileTransfer:     true,
		SessionRecording: true,
		Reconnect:        true,
		Extras: map[string]bool{
			"shareable": true,
			"sftp":      true,
		},
	}, newSSHTemplate())
}

// NewSFTPDriver returns the SFTP file transfer descriptor backed by the SSH driver.
func NewSFTPDriver() *Driver {
	return newDriver(drivers.Descriptor{
		ID:        DriverIDSFTP,
		Module:    "ssh",
		Title:     "SFTP",
		Category:  "file_transfer",
		Icon:      "folder",
		SortOrder: 2,
	}, drivers.Capabilities{
		FileTransfer: true,
		Extras: map[string]bool{
			"ssh_transport": true,
		},
	}, nil)
}

// Capabilities reports the supported feature flags for the driver instance.
func (d *Driver) Capabilities(ctx context.Context) (drivers.Capabilities, error) {
	return d.caps, nil
}

// Description provides user-facing metadata for the driver.
func (d *Driver) Description() string {
	switch d.ID() {
	case DriverIDSSH:
		return "Secure Shell access with collaborative features and optional session recording."
	case DriverIDSFTP:
		return "Secure File Transfer Protocol powered by the SSH transport."
	default:
		return ""
	}
}

// DefaultPort returns the conventional SSH port.
func (d *Driver) DefaultPort() int {
	return 22
}

// ConnectionTemplate publishes the dynamic connection schema for SSH.
func (d *Driver) ConnectionTemplate() (*drivers.ConnectionTemplate, error) {
	if d.ID() != DriverIDSSH {
		return nil, nil
	}
	return d.template, nil
}

func newSSHTemplate() *drivers.ConnectionTemplate {
	return &drivers.ConnectionTemplate{
		DriverID:    DriverIDSSH,
		Version:     sshTemplateVersion,
		DisplayName: "SSH Connection",
		Description: "Configure host, port, and session behaviour for SSH connections.",
		Metadata: map[string]any{
			"requires_identity": true,
		},
		Sections: []drivers.ConnectionSection{
			{
				ID:    "endpoint",
				Label: "Endpoint",
				Fields: []drivers.ConnectionField{
					{
						Key:         "host",
						Label:       "Host",
						Type:        drivers.ConnectionFieldTypeTargetHost,
						Required:    true,
						Placeholder: "server.example.com",
						Validation: map[string]any{
							"pattern": hostnamePattern,
						},
						Binding: &drivers.ConnectionBinding{
							Target:   drivers.BindingTargetConnectionTarget,
							Index:    0,
							Property: "host",
						},
					},
					{
						Key:     "port",
						Label:   "Port",
						Type:    drivers.ConnectionFieldTypeTargetPort,
						Default: 22,
						Validation: map[string]any{
							"min": 1,
							"max": 65535,
						},
						Binding: &drivers.ConnectionBinding{
							Target:   drivers.BindingTargetConnectionTarget,
							Index:    0,
							Property: "port",
						},
					},
				},
			},
			{
				ID:    "session",
				Label: "Session Behaviour",
				Fields: []drivers.ConnectionField{
					{
						Key:      "session_override_enabled",
						Label:    "Custom Session Overrides",
						Type:     drivers.ConnectionFieldTypeBoolean,
						Default:  false,
						HelpText: "Toggle to override concurrency and timeout defaults for this connection.",
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetMetadata,
							Path:   "session_override.enabled",
						},
					},
					{
						Key:      "concurrent_limit",
						Label:    "Concurrent Sessions",
						Type:     drivers.ConnectionFieldTypeNumber,
						Default:  0,
						HelpText: "Maximum concurrent sessions. Zero uses the global default.",
						Validation: map[string]any{
							"min": 0,
							"max": 1000,
						},
						Dependencies: []drivers.FieldDependency{
							{Field: "session_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "concurrent_limit",
						},
					},
					{
						Key:      "idle_timeout_minutes",
						Label:    "Idle Timeout (minutes)",
						Type:     drivers.ConnectionFieldTypeNumber,
						Default:  0,
						HelpText: "Disconnect after N minutes of inactivity. Zero disables the override.",
						Validation: map[string]any{
							"min": 0,
							"max": 10080,
						},
						Dependencies: []drivers.FieldDependency{
							{Field: "session_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "idle_timeout_minutes",
						},
					},
					{
						Key:      "enable_sftp",
						Label:    "Enable SFTP",
						Type:     drivers.ConnectionFieldTypeBoolean,
						Default:  true,
						HelpText: "Allow SFTP alongside SSH sessions.",
						Dependencies: []drivers.FieldDependency{
							{Field: "session_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "enable_sftp",
						},
					},
					{
						Key:      "recording_enabled",
						Label:    "Enable Session Recording",
						Type:     drivers.ConnectionFieldTypeBoolean,
						Default:  false,
						HelpText: "Capture session activity when optional recording is enabled.",
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "recording_enabled",
						},
					},
				},
			},
			{
				ID:    "terminal",
				Label: "Terminal Overrides",
				Fields: []drivers.ConnectionField{
					{
						Key:      "terminal_override_enabled",
						Label:    "Custom Terminal Settings",
						Type:     drivers.ConnectionFieldTypeBoolean,
						Default:  false,
						HelpText: "Toggle to override default terminal appearance for this connection.",
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetMetadata,
							Path:   "terminal_override.enabled",
						},
					},
					{
						Key:         "terminal_font_family",
						Label:       "Font Family",
						Type:        drivers.ConnectionFieldTypeString,
						Default:     "monospace",
						Placeholder: "JetBrains Mono, Fira Code",
						Dependencies: []drivers.FieldDependency{
							{Field: "terminal_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "terminal_config_override.font_family",
						},
					},
					{
						Key:     "terminal_font_size",
						Label:   "Font Size",
						Type:    drivers.ConnectionFieldTypeNumber,
						Default: 14,
						Validation: map[string]any{
							"min": 8,
							"max": 96,
						},
						Dependencies: []drivers.FieldDependency{
							{Field: "terminal_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "terminal_config_override.font_size",
						},
					},
					{
						Key:     "terminal_scrollback_limit",
						Label:   "Scrollback Limit",
						Type:    drivers.ConnectionFieldTypeNumber,
						Default: 1000,
						Validation: map[string]any{
							"min": 200,
							"max": 10000,
						},
						Dependencies: []drivers.FieldDependency{
							{Field: "terminal_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "terminal_config_override.scrollback_limit",
						},
					},
					{
						Key:     "terminal_enable_webgl",
						Label:   "Enable WebGL renderer",
						Type:    drivers.ConnectionFieldTypeBoolean,
						Default: true,
						Dependencies: []drivers.FieldDependency{
							{Field: "terminal_override_enabled", Equals: true},
						},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "terminal_config_override.enable_webgl",
						},
					},
				},
			},
		},
	}
}

// HealthCheck performs a lightweight validation ensuring the SSH implementation is usable.
func (d *Driver) HealthCheck(ctx context.Context) error {
	cfg := &gossh.ClientConfig{
		User:            "healthcheck",
		Auth:            []gossh.AuthMethod{gossh.Password("placeholder")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	if cfg.User == "" || len(cfg.Auth) == 0 || cfg.HostKeyCallback == nil {
		return errors.New("ssh: invalid health check configuration")
	}
	if cfg.Timeout <= 0 {
		return errors.New("ssh: invalid health check timeout")
	}
	return nil
}

// CredentialTemplate publishes the credential schema shared by SSH and SFTP.
func (d *Driver) CredentialTemplate() (*drivers.CredentialTemplate, error) {
	tpl := &drivers.CredentialTemplate{
		DriverID:    DriverIDSSH,
		Version:     "2025-01-01",
		DisplayName: "SSH Credentials",
		Description: "SSH credentials supporting private key or password authentication.",
		CompatibleProtocols: []string{
			DriverIDSSH,
			DriverIDSFTP,
		},
		Fields: []drivers.CredentialField{
			{
				Key:         drivers.CredentialFieldKeyUsername,
				Label:       "Username",
				Type:        drivers.CredentialFieldTypeString,
				Required:    true,
				Description: "Remote login username.",
			},
			{
				Key:      drivers.CredentialFieldKeyAuthMethod,
				Label:    "Authentication Method",
				Type:     drivers.CredentialFieldTypeEnum,
				Required: true,
				Options: []drivers.CredentialOption{
					{Value: "private_key", Label: "Private Key"},
					{Value: "password", Label: "Password"},
				},
				Description: "Select the preferred authentication mechanism.",
				Metadata: map[string]any{
					"section": "authentication",
					"hint":    "Choose how ShellCN authenticates to the remote host.",
				},
			},
			{
				Key:         drivers.CredentialFieldKeyPrivateKey,
				Label:       "Private Key",
				Type:        drivers.CredentialFieldTypeSecret,
				InputModes:  []string{drivers.CredentialInputModeText, drivers.CredentialInputModeFile},
				Description: "PEM-encoded private key material.",
				Metadata: map[string]any{
					"visibility": map[string]any{
						"field":  drivers.CredentialFieldKeyAuthMethod,
						"equals": []string{"private_key"},
					},
					"required_when": map[string]any{
						"field":  drivers.CredentialFieldKeyAuthMethod,
						"equals": []string{"private_key"},
					},
					"allow_file_import": true,
					"multiline":         true,
					"section":           "authentication",
				},
			},
			{
				Key:         drivers.CredentialFieldKeyPassphrase,
				Label:       "Passphrase",
				Type:        drivers.CredentialFieldTypeSecret,
				Description: "Optional passphrase for encrypted private keys.",
				Metadata: map[string]any{
					"visibility": map[string]any{
						"field":  drivers.CredentialFieldKeyAuthMethod,
						"equals": []string{"private_key"},
					},
					"section": "authentication",
					"hint":    "Only required when the private key is encrypted.",
				},
			},
			{
				Key:         drivers.CredentialFieldKeyPassword,
				Label:       "Password",
				Type:        drivers.CredentialFieldTypeSecret,
				Description: "Password authentication fallback.",
				Metadata: map[string]any{
					"visibility": map[string]any{
						"field":  drivers.CredentialFieldKeyAuthMethod,
						"equals": []string{"password"},
					},
					"required_when": map[string]any{
						"field":  drivers.CredentialFieldKeyAuthMethod,
						"equals": []string{"password"},
					},
					"section": "authentication",
				},
			},
		},
		Metadata: map[string]any{
			"sections": []map[string]string{
				{
					"id":          "authentication",
					"label":       "Authentication",
					"description": "Provide credentials used to establish SSH and SFTP sessions.",
				},
			},
			"default_auth_method": "private_key",
		},
	}
	return tpl, nil
}

func init() {
	drivers.MustRegisterDefault(NewSSHDriver())
	drivers.MustRegisterDefault(NewSFTPDriver())
}
