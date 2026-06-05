// Package sftp implements the file-only SFTP protocol plugin.
package sftp

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/sshsftp"
	"github.com/charlesng35/shellcn/sdk/plugin"
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
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "server"},
		Category:            plugin.CategoryFiles,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSingle,
		Tabs:                []plugin.Panel{filesTab()},
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

func filesTab() plugin.Panel {
	return plugin.Panel{
		Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "folder"},
		Type:   plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: "sftp.sftp.list", Params: map[string]string{"path": "."}},
		Config: plugin.FileBrowserConfig{
			PathParam:       "path",
			ReadRouteID:     "sftp.sftp.read",
			DownloadRouteID: "sftp.sftp.download",
			WriteRouteID:    "sftp.sftp.write",
			UploadRouteID:   "sftp.sftp.upload",
			MkdirRouteID:    "sftp.sftp.mkdir",
			RenameRouteID:   "sftp.sftp.rename",
			DeleteRouteID:   "sftp.sftp.delete",
			MoveRouteID:     "sftp.sftp.move",
			CopyRouteID:     "sftp.sftp.copy",
			ChmodRouteID:    "sftp.sftp.chmod",
			ArchiveRouteID:  "sftp.sftp.archive",
			Writable:        true,
			MultipleUpload:  true,
			MaxUploadBytes:  52428800,
			UploadFieldName: "files",
		},
	}
}
