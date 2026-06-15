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
		Streams:             []plugin.Stream{{ID: "sftp.sftp.jobs", Kind: plugin.StreamFileJob, RouteID: "sftp.sftp.jobs"}},
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
				Kind: sshsftp.CredentialKindSSHPassword, Protocols: []string{"sftp"},
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "stored_password"}}}},
			{Key: sshsftp.CredentialPrivateKeyField, Label: "Stored SSH private key", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
				Kind: sshsftp.CredentialKindSSHPrivateKey, Protocols: []string{"sftp"},
			}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "stored_private_key"}}}},
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
			PathParam: "path",
			Routes: plugin.FileBrowserRoutes{
				Read:     "sftp.sftp.read",
				Download: "sftp.sftp.download",
				Write:    "sftp.sftp.write",
				Mkdir:    "sftp.sftp.mkdir",
				Rename:   "sftp.sftp.rename",
				Delete:   "sftp.sftp.delete",
				Chmod:    "sftp.sftp.chmod",
				Archive:  "sftp.sftp.archive",
			},
			Upload: plugin.FileUploadConfig{
				RouteID:   "sftp.sftp.upload",
				FieldName: "files",
				Multiple:  true,
			},
			Writable: true,
			Jobs: &plugin.FileJobConfig{
				Source:     &plugin.DataSource{RouteID: "sftp.sftp.jobs", Method: plugin.MethodWS},
				Operations: []plugin.FileJobOperation{plugin.FileJobMove, plugin.FileJobCopy},
			},
		},
	}
}
