// Package vnc implements the VNC remote-desktop plugin. The gateway performs
// RFB authentication to the upstream server (keeping the password server-side)
// and streams the raw RFB session to the browser's noVNC client.
package vnc

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// iconSVG is the Lucide "monitor" glyph, rendered via the sanitized inline-SVG
// icon path.
const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8"/><path d="M12 17v4"/></svg>`

// Plugin exposes a VNC console rendered by noVNC.
type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "vnc",
		Version:             "0.1.0",
		Title:               "VNC",
		Description:         "VNC remote desktop rendered with noVNC.",
		Icon:                plugin.Icon{Type: plugin.IconSVG, Value: iconSVG},
		Category:            plugin.CategoryRemoteDesktop,
		Config:              configSchema("vnc"),
		Capabilities:        []plugin.Capability{"remote_desktop"},
		CredentialKinds:     credentialKinds(),
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs: []plugin.Tab{{
			Key:    "console",
			Label:  "Console",
			Icon:   plugin.Icon{Type: plugin.IconSVG, Value: iconSVG},
			Panel:  plugin.PanelRemoteDesktop,
			Source: &plugin.DataSource{RouteID: "vnc.desktop", Method: plugin.MethodWS},
			Config: plugin.RemoteDesktopConfig{Resize: true},
		}},
		Streams: []plugin.Stream{{ID: "vnc.desktop", Kind: plugin.StreamDesktop, RouteID: "vnc.desktop"}},
		Recording: []plugin.RecordingCapability{{
			Class:     plugin.RecordingDesktop,
			Formats:   []plugin.RecordingFormat{plugin.FormatWebMCanvas},
			StreamIDs: []string{"vnc.desktop"},
		}},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return []plugin.Route{{
		ID:         "vnc.desktop",
		Method:     plugin.MethodWS,
		Path:       "/desktop",
		Permission: "vnc.desktop",
		Risk:       plugin.RiskPrivileged,
		AuditEvent: "vnc.desktop",
		Stream:     desktopStream,
	}}
}

func (p *Plugin) Connect(_ context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return connect(cfg)
}
