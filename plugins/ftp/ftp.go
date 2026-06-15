// Package ftp implements the FTP filesystem plugin.
package ftp

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/plugins/shared/ftpfs"
	"github.com/charlesng35/shellcn/sdk/plugin"
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
		Layout:              plugin.LayoutSingle,
		Streams:             filesystem.Streams(protocolName),
		Tabs: []plugin.Panel{filesystem.FilesTab(
			protocolName,
			filesystem.WithMove(protocolName),
			filesystem.WithCopy(protocolName),
			filesystem.WithArchive(protocolName),
		)},
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
			{Key: "passive_port_start", Label: "Passive port start", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, Help: "Optional start of the passive data port range allowed for file transfers."},
			{Key: "passive_port_end", Label: "Passive port end", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 65535}}, Help: "Optional end of the passive data port range allowed for file transfers."},
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
		{Key: "credential_id", Label: "FTP credential", Type: plugin.FieldCredentialRef, Required: true, Credential: &plugin.CredentialSelector{
			Kind: plugin.CredentialKindBasicAuth, Protocols: []string{protocolName},
		}, VisibleWhen: &plugin.Condition{AllOf: []plugin.Rule{{Field: "auth", Op: plugin.OpEq, Value: "credential"}}}},
	}
}
