// Package ssh implements the full SSH protocol plugin.
package ssh

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/sshsftp"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Plugin exposes full SSH: terminal, files, and command snippets.
type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "ssh",
		Version:             "0.1.0",
		Title:               "SSH",
		Description:         "Secure shell with terminal, SFTP files, and command snippets.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "terminal"},
		Category:            plugin.CategoryShell,
		Config:              configSchema("ssh"),
		Capabilities:        []plugin.Capability{"terminal", "filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs: []plugin.Panel{
			terminalTab(),
			filesTab("ssh"),
			snippetsTab(),
		},
		Actions: []plugin.Action{
			{
				ID:      "ssh.snippet.create",
				Label:   "New snippet",
				Icon:    plugin.Icon{Type: plugin.IconLucide, Value: "plus"},
				RouteID: "ssh.snippet.create",
			},
			{
				ID:          "ssh.snippet.run",
				Label:       "Run",
				Icon:        plugin.Icon{Type: plugin.IconLucide, Value: "play"},
				RouteID:     "ssh.snippet.run",
				Params:      map[string]string{"id": "${record.id}"},
				Confirm:     true,
				ConfirmText: "Run this snippet on the SSH host?",
				OnSuccess: &plugin.ActionSuccess{
					SelectTab: "terminal",
					Effects: []plugin.ActionEffect{{
						Type: plugin.ActionEffectTerminalInput,
						TerminalInput: &plugin.TerminalInputEffect{
							Tab:           "terminal",
							ResultField:   "command",
							AppendNewline: true,
						},
					}},
				},
			},
			{
				ID:          "ssh.snippet.delete",
				Label:       "Delete",
				Icon:        plugin.Icon{Type: plugin.IconLucide, Value: "trash"},
				RouteID:     "ssh.snippet.delete",
				Params:      map[string]string{"id": "${record.id}"},
				Confirm:     true,
				ConfirmText: "Delete this snippet?",
			},
		},
		Streams: []plugin.Stream{
			{ID: "ssh.shell", Kind: plugin.StreamTerminal, RouteID: "ssh.shell"},
			{ID: "ssh.sftp.transfer", Kind: plugin.StreamFileTransfer, RouteID: "ssh.sftp.transfer"},
		},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2},
			StreamIDs: []string{"ssh.shell"}, Authoritative: true,
		}},
	}
}

func terminalTab() plugin.Panel {
	return plugin.Panel{
		Key:   "terminal",
		Label: "Terminal",
		Icon:  plugin.Icon{Type: plugin.IconLucide, Value: "terminal"},
		Type:  plugin.PanelTerminal,
		Source: &plugin.DataSource{
			RouteID: "ssh.shell",
			Method:  plugin.MethodWS,
			Params:  map[string]string{"cols": "80", "rows": "24"},
		},
		Config: plugin.TerminalConfig{Zoom: true, Search: true},
		Variants: []plugin.PanelVariant{{
			Type: plugin.PanelTerminalGrid,
			Config: plugin.TerminalGridConfig{
				MaxPanes:     6,
				DefaultPanes: 1,
				Zoom:         true,
				Search:       true,
			},
			VisibleWhen: &plugin.Condition{
				AllOf: []plugin.Rule{{
					Field: "terminal_layout",
					Op:    plugin.OpEq,
					Value: "grid",
				}},
			},
		}},
	}
}

func snippetsTab() plugin.Panel {
	return plugin.Panel{
		Key: "snippets", Label: "Snippets", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "code"},
		Type: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ssh.snippet.list"},
		Config: plugin.TableConfig{
			Columns: []plugin.Column{
				{Key: "name", Label: "Name", Sortable: true},
				{Key: "body", Label: "Command"},
				{Key: "updatedAt", Label: "Updated", Type: plugin.ColumnDateTime, Sortable: true},
			},
			ActionIDs:    []string{"ssh.snippet.create"},
			RowActionIDs: []string{"ssh.snippet.run", "ssh.snippet.delete"},
			EmptyText:    "No snippets. Create one for a command you run often.",
		},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return sshsftp.Routes("ssh", "ssh", true)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return sshsftp.Connect(ctx, cfg)
}

func configSchema(protocol string) plugin.Schema {
	inlineAuth := plugin.Condition{AnyOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}, {Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: 22, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "user", Label: "Username", Type: plugin.FieldText, Required: true, Default: "root", VisibleWhen: &inlineAuth},
			{Key: "host_key_verification", Label: "Host key verification", Type: plugin.FieldSelect, Required: true, Default: "pinned", Options: []plugin.Option{
				{Label: "Verify pinned host key", Value: "pinned"},
				{Label: "Do not verify (unsafe)", Value: "insecure"},
			}, Help: "Use a pinned OpenSSH public key, known_hosts line, or SHA256 fingerprint. Disable verification only for disposable or already isolated hosts."},
			{Key: "host_key", Label: "Pinned host key", Type: plugin.FieldTextarea, Required: true, Placeholder: "SHA256:...", Help: "Paste the server public host key, a known_hosts line, or its SHA256 fingerprint.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "host_key_verification", Op: plugin.OpEq, Value: "pinned"}}}},
		}},
		{Name: "Auth", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
				{Label: "Password", Value: "password"},
				{Label: "Private key", Value: "private_key"},
				{Label: "Stored SSH password", Value: "stored_password"},
				{Label: "Stored SSH private key", Value: "stored_private_key"},
			}},
			{Key: sshsftp.CredentialPasswordField, Label: "Stored SSH password", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: sshsftp.CredentialKindSSHPassword, Protocols: []string{protocol},
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "stored_password"}}}},
			{Key: sshsftp.CredentialPrivateKeyField, Label: "Stored SSH private key", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: sshsftp.CredentialKindSSHPrivateKey, Protocols: []string{protocol},
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "stored_private_key"}}}},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
			{Key: "private_key", Label: "Private key", Type: plugin.FieldTextarea, Required: true, Secret: true, Help: "PEM-encoded private key.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
			{Key: "passphrase", Label: "Key passphrase", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
		}},
		{Name: "Terminal", Fields: []plugin.Field{
			{Key: "terminal_layout", Label: "Terminal layout", Type: plugin.FieldSelect, Required: true, Default: "single", Options: []plugin.Option{
				{Label: "Single terminal", Value: "single"},
				{Label: "Terminal grid", Value: "grid"},
			}, Help: "Use a single terminal by default. Enable grid only when you need multiple concurrent terminal sessions."},
		}},
	}}
}

func filesTab(prefix string) plugin.Panel {
	return plugin.Panel{
		Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "folder"},
		Type:   plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: prefix + ".sftp.list", Params: map[string]string{"path": "."}},
		Config: plugin.FileBrowserConfig{
			PathParam: "path",
			Routes: plugin.FileBrowserRoutes{
				Read:     prefix + ".sftp.read",
				Download: prefix + ".sftp.download",
				Write:    prefix + ".sftp.write",
				Mkdir:    prefix + ".sftp.mkdir",
				Rename:   prefix + ".sftp.rename",
				Delete:   prefix + ".sftp.delete",
				Chmod:    prefix + ".sftp.chmod",
				Archive:  prefix + ".sftp.archive",
			},
			Upload: plugin.FileUploadConfig{
				RouteID:   prefix + ".sftp.upload",
				FieldName: "files",
				Multiple:  true,
				MaxBytes:  52428800,
			},
			Writable: true,
			Transfer: &plugin.FileTransferConfig{
				Source:     &plugin.DataSource{RouteID: prefix + ".sftp.transfer", Method: plugin.MethodWS},
				Operations: []plugin.FileTransferOperation{plugin.FileTransferMove, plugin.FileTransferCopy},
			},
		},
	}
}
