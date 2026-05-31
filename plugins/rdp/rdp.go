// Package rdp implements the RDP remote-desktop plugin. grdp speaks the RDP
// protocol in pure Go and authenticates server-side, so the password never
// reaches the browser; its decoded framebuffer is bridged to a synthetic RFB
// session that the browser renders with noVNC.
package rdp

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// iconSVG is the Lucide "app-window" glyph, rendered via the sanitized inline-SVG
// icon path.
const iconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M2 8h20"/><rect width="20" height="16" x="2" y="4" rx="2"/><path d="M6 4v4"/><path d="M10 4v4"/></svg>`

// Plugin exposes a Windows RDP console rendered by noVNC.
type Plugin struct{}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Manifest() plugin.Manifest {
	icon := plugin.Icon{Type: plugin.IconSVG, Value: iconSVG}
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "rdp",
		Version:             "0.1.0",
		Title:               "RDP",
		Description:         "Windows Remote Desktop rendered with noVNC.",
		Icon:                icon,
		Category:            plugin.CategoryRemoteDesktop,
		Config:              configSchema("rdp"),
		Capabilities:        []plugin.Capability{"remote_desktop"},
		CredentialKinds:     credentialKinds(),
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutSingle,
		Tabs: []plugin.Panel{{
			Key:    "console",
			Label:  "Console",
			Icon:   icon,
			Type:   plugin.PanelRemoteDesktop,
			Source: &plugin.DataSource{RouteID: "rdp.desktop", Method: plugin.MethodWS},
			Config: plugin.RemoteDesktopConfig{Resize: true},
		}},
		Streams: []plugin.Stream{{ID: "rdp.desktop", Kind: plugin.StreamDesktop, RouteID: "rdp.desktop"}},
		Recording: []plugin.RecordingCapability{{
			Class:     plugin.RecordingDesktop,
			Formats:   []plugin.RecordingFormat{plugin.FormatWebMCanvas},
			StreamIDs: []string{"rdp.desktop"},
		}},
	}
}

func (p *Plugin) Routes() []plugin.Route {
	return []plugin.Route{{
		ID:         "rdp.desktop",
		Method:     plugin.MethodWS,
		Path:       "/desktop",
		Permission: "rdp.desktop",
		Risk:       plugin.RiskPrivileged,
		AuditEvent: "rdp.desktop",
		Stream:     desktopStream,
	}}
}

func (p *Plugin) Connect(_ context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return connect(cfg)
}
