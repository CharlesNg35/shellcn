// Package sftp implements the file-only SFTP protocol plugin.
package sftp

import (
	"context"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/sshsftp"
)

// Plugin exposes file-only SFTP access over SSH.
type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "sftp",
		Version:             "0.1.0",
		Title:               "SFTP",
		Description:         "File browser over SSH SFTP.",
		Icon:                plugin.Icon{Type: plugin.IconName, Value: "server"},
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs:                []plugin.Tab{filesTab()},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return sshsftp.Routes("sftp", "sftp", false)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return sshsftp.Connect(ctx, cfg)
}

func configSchema() plugin.Schema {
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
				Kinds: []plugin.CredentialKind{sshsftp.CredentialSSHPrivateKey, sshsftp.CredentialSSHPassword}, Protocols: []string{"sftp"}, Required: true,
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
			{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
			{Key: "private_key", Label: "Private key", Type: plugin.FieldTextarea, Required: true, Secret: true, Help: "PEM-encoded private key.", VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
			{Key: "passphrase", Label: "Key passphrase", Type: plugin.FieldPassword, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "private_key"}}}},
		}},
	}}
}

func filesTab() plugin.Tab {
	return plugin.Tab{
		Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconName, Value: "folder"},
		Panel:  plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: "sftp.sftp.list", Params: map[string]string{"path": "."}},
		Config: map[string]any{
			"pathParam":       "path",
			"readRouteId":     "sftp.sftp.read",
			"downloadRouteId": "sftp.sftp.download",
			"writeRouteId":    "sftp.sftp.write",
			"uploadRouteId":   "sftp.sftp.upload",
			"mkdirRouteId":    "sftp.sftp.mkdir",
			"renameRouteId":   "sftp.sftp.rename",
			"deleteRouteId":   "sftp.sftp.delete",
			"writable":        true,
			"multipleUpload":  true,
			"maxUploadBytes":  52428800,
			"uploadFieldName": "files",
		},
	}
}
