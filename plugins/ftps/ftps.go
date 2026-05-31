// Package ftps implements the FTP over TLS filesystem plugin.
package ftps

import (
	"context"
	"fmt"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/plugins/shared/ftpfs"
)

const protocolName = "ftps"

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "FTPS",
		Description:         "File browser for FTP servers secured with TLS.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "folder-lock"},
		Category:            plugin.CategoryFiles,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs:                []plugin.Panel{filesystem.FilesTab(protocolName)},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return filesystem.Routes(protocolName, protocolName)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	mode := ftpfs.TLSMode(cfg.String("tls_mode"))
	if mode == "" {
		mode = ftpfs.TLSExplicit
	}
	if mode != ftpfs.TLSExplicit && mode != ftpfs.TLSImplicit {
		return nil, fmt.Errorf("%w: unsupported tls mode %q", plugin.ErrInvalidInput, mode)
	}
	return ftpfs.Connect(ctx, cfg, ftpfs.Options{TLSMode: mode, VerifyTLS: true})
}

func configSchema() plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "files.example.com"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: ftpfs.DefaultFTPPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "root_path", Label: "Root path", Type: plugin.FieldText, Default: "/", Placeholder: "/"},
			{Key: "passive_port_start", Label: "Passive port start", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, Help: "Optional start of the passive data port range allowed for file transfers."},
			{Key: "passive_port_end", Label: "Passive port end", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, Help: "Optional end of the passive data port range allowed for file transfers."},
			{Key: "tls_mode", Label: "TLS mode", Type: plugin.FieldSelect, Required: true, Default: string(ftpfs.TLSExplicit), Options: []plugin.Option{
				{Label: "Explicit TLS", Value: string(ftpfs.TLSExplicit)},
				{Label: "Implicit TLS", Value: string(ftpfs.TLSImplicit)},
			}},
			{Key: "verify_tls", Label: "Verify TLS certificate", Type: plugin.FieldToggle, Default: true},
		}},
		{Name: "Authentication", Fields: authFields()},
	}}
}

func authFields() []plugin.Field {
	return []plugin.Field{
		{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
			{Label: "Username & password", Value: "password"},
			{Label: "Stored FTPS credential", Value: "credential"},
			{Label: "Anonymous FTP", Value: "anonymous"},
		}},
		{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "credential_id", Label: "FTPS credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
			Kinds: []plugin.CredentialKind{plugin.CredentialBasicAuth}, Protocols: []string{protocolName}, Required: true,
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
	}
}
