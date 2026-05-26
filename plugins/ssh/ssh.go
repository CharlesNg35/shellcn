// Package ssh implements the full SSH protocol plugin.
package ssh

import (
	"context"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/sshsftp"
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
		Icon:                plugin.Icon{Type: plugin.IconName, Value: "terminal"},
		Config:              configSchema("ssh"),
		Capabilities:        []plugin.Capability{"terminal", "filesystem"},
		CredentialKinds:     sshsftp.CredentialKinds(),
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs: []plugin.Tab{
			{
				Key: "terminal", Label: "Terminal", Icon: plugin.Icon{Type: plugin.IconName, Value: "terminal"},
				Panel: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "ssh.shell", Method: plugin.MethodWS, Params: map[string]string{"cols": "80", "rows": "24"}},
			},
			filesTab("ssh"),
			snippetsTab(),
		},
		Actions: []plugin.Action{
			{
				ID: "ssh.snippet.create", Label: "New snippet", Icon: plugin.Icon{Type: plugin.IconName, Value: "plus"},
				RouteID: "ssh.snippet.create",
			},
			{
				ID: "ssh.snippet.run", Label: "Run", Icon: plugin.Icon{Type: plugin.IconName, Value: "play"},
				RouteID: "ssh.snippet.run", Params: map[string]string{"id": "${resource.uid}"},
				Confirm: true, ConfirmText: "Run this snippet on the SSH host?",
			},
			{
				ID: "ssh.snippet.delete", Label: "Delete", Icon: plugin.Icon{Type: plugin.IconName, Value: "trash"},
				RouteID: "ssh.snippet.delete", Params: map[string]string{"id": "${resource.uid}"},
				Confirm: true, ConfirmText: "Delete this snippet?",
			},
		},
		Streams: []plugin.Stream{{ID: "ssh.shell", Kind: plugin.StreamTerminal, RouteID: "ssh.shell"}},
		Recording: []plugin.RecordingCapability{{
			Class: plugin.RecordingTerminal, Formats: []plugin.RecordingFormat{plugin.FormatAsciicastV2},
			StreamIDs: []string{"ssh.shell"}, Authoritative: true,
		}},
	}
}

func snippetsTab() plugin.Tab {
	return plugin.Tab{
		Key: "snippets", Label: "Snippets", Icon: plugin.Icon{Type: plugin.IconName, Value: "code"},
		Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ssh.snippet.list"},
		Config: plugin.TableConfig{
			Columns: []plugin.Column{
				{Key: "name", Label: "Name", Sortable: true},
				{Key: "body", Label: "Command"},
				{Key: "updatedAt", Label: "Updated", Type: plugin.ColumnDateTime, Sortable: true},
			},
			ActionIDs:    []string{"ssh.snippet.create"},
			RowActionIDs: []string{"ssh.snippet.run", "ssh.snippet.delete"},
		}.Map(),
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return sshsftp.Routes("ssh", "ssh", true)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return sshsftp.Connect(ctx, cfg)
}

func configSchema(protocol string) plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Basic", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "10.0.0.1"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: 22, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "user", Label: "Username", Type: plugin.FieldText, Required: true, Default: "root"},
		}},
		{Name: "Auth", Fields: []plugin.Field{
			{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
				{Label: "Password", Value: "password"},
				{Label: "Private key", Value: "private_key"},
				{Label: "Stored credential", Value: "credential"},
			}},
			{Key: "credential_id", Label: "Credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
				Kinds: []plugin.CredentialKind{sshsftp.CredentialSSHPrivateKey, sshsftp.CredentialSSHPassword}, Protocols: []string{protocol}, Required: true,
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
			{Key: "private_key", Label: "Private key", Type: plugin.FieldTextarea, Required: true, Secret: true, Help: "PEM-encoded private key.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
			{Key: "passphrase", Label: "Key passphrase", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
		}},
	}}
}

func filesTab(prefix string) plugin.Tab {
	return plugin.Tab{
		Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconName, Value: "folder"},
		Panel:  plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: prefix + ".sftp.list", Params: map[string]string{"path": "."}},
		Config: map[string]any{
			"pathParam":       "path",
			"readRouteId":     prefix + ".sftp.read",
			"downloadRouteId": prefix + ".sftp.download",
			"writeRouteId":    prefix + ".sftp.write",
			"uploadRouteId":   prefix + ".sftp.upload",
			"mkdirRouteId":    prefix + ".sftp.mkdir",
			"renameRouteId":   prefix + ".sftp.rename",
			"deleteRouteId":   prefix + ".sftp.delete",
			"writable":        true,
			"multipleUpload":  true,
			"maxUploadBytes":  52428800,
			"uploadFieldName": "files",
		},
	}
}
