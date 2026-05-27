// Package ftp implements the FTP filesystem plugin.
package ftp

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/plugins/shared/ftpfs"
)

const protocolName = "ftp"

type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                protocolName,
		Version:             "0.1.0",
		Title:               "FTP",
		Description:         "File browser for FTP servers.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "folder-sync"},
		Category:            plugin.CategoryFiles,
		Config:              configSchema(),
		Capabilities:        []plugin.Capability{"filesystem"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs:                []plugin.Tab{filesystem.FilesTab(protocolName)},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return filesystem.Routes(protocolName, protocolName)
}

func (p *Plugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return ftpfs.Connect(ctx, cfg, ftpfs.Options{TLSMode: ftpfs.TLSNone})
}

func configSchema() plugin.Schema {
	return plugin.Schema{Groups: []plugin.Group{
		{Name: "Server", Fields: []plugin.Field{
			{Key: "host", Label: "Host", Type: plugin.FieldText, Required: true, Placeholder: "files.example.com"},
			{Key: "port", Label: "Port", Type: plugin.FieldNumber, Default: ftpfs.DefaultFTPPort, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}},
			{Key: "root_path", Label: "Root path", Type: plugin.FieldText, Default: "/", Placeholder: "/"},
		}},
		{Name: "Authentication", Fields: authFields()},
	}}
}

func authFields() []plugin.Field {
	return []plugin.Field{
		{Key: "auth", Label: "Authentication", Type: plugin.FieldSelect, Required: true, Default: "password", Options: []plugin.Option{
			{Label: "Username & password", Value: "password"},
			{Label: "Stored FTP credential", Value: "credential"},
			{Label: "Anonymous FTP", Value: "anonymous"},
		}},
		{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "password", Label: "Password", Type: plugin.FieldPassword, Required: true, Secret: true, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "password"}}}},
		{Key: "credential_id", Label: "FTP credential", Type: plugin.FieldCredentialRef, Credential: &plugin.CredentialSelector{
			Kinds: []plugin.CredentialKind{plugin.CredentialBasicAuth}, Protocols: []string{protocolName}, Required: true,
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
	}
}
