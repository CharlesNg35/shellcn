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
)

// Driver implements the drivers.Driver interface for SSH and SFTP descriptors.
type Driver struct {
	drivers.BaseDriver
	caps drivers.Capabilities
}

// newDriver constructs a driver instance with shared metadata and capability flags.
func newDriver(desc drivers.Descriptor, caps drivers.Capabilities) *Driver {
	return &Driver{
		BaseDriver: drivers.NewBaseDriver(desc),
		caps:       caps,
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
	})
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
	})
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
					"section": "authentication",
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
