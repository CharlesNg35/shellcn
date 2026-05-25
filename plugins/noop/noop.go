// Package noop is a trivial reference plugin that exercises the whole core path:
// manifest projection, a paginated list route, and a WebSocket echo stream. It
// has no real upstream — Connect returns an in-memory session.
package noop

import (
	"context"
	"io"

	"github.com/charlesng/shellcn/internal/plugin"
)

// Plugin is the stateless noop singleton.
type Plugin struct{}

// New returns the plugin singleton.
func New() *Plugin { return &Plugin{} }

// Manifest declares a flat layout with an Items table and an Echo terminal.
func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "noop",
		Version:             "0.1.0",
		Title:               "Noop",
		Description:         "A trivial plugin that proves the core runtime end to end.",
		Icon:                plugin.Icon{Type: plugin.IconName, Value: "box"},
		Capabilities:        []plugin.Capability{"terminal"},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Tabs: []plugin.Tab{
			{
				Key: "items", Label: "Items", Icon: plugin.Icon{Type: plugin.IconName, Value: "list"},
				Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "noop.list"},
			},
			{
				Key: "echo", Label: "Echo", Icon: plugin.Icon{Type: plugin.IconName, Value: "terminal"},
				Panel: plugin.PanelTerminal, Source: &plugin.DataSource{RouteID: "noop.echo", Method: plugin.MethodWS},
			},
		},
		Streams: []plugin.Stream{{ID: "noop.echo", Kind: plugin.StreamTerminal, RouteID: "noop.echo"}},
	}
}

// Routes exposes a paginated list and a WS echo stream.
func (p *Plugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "noop.list", Method: plugin.MethodGet, Path: "/items",
			Permission: "noop.read", Risk: plugin.RiskSafe, AuditEvent: "noop.list",
			Handle: p.list,
		},
		{
			ID: "noop.echo", Method: plugin.MethodWS, Path: "/echo",
			Permission: "noop.read", Risk: plugin.RiskSafe, AuditEvent: "noop.echo",
			Stream: p.echo,
		},
	}
}

// Connect returns an in-memory session (no real upstream).
func (p *Plugin) Connect(_ context.Context, _ plugin.ConnectConfig) (plugin.Session, error) {
	return &noopSession{}, nil
}

type item struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (p *Plugin) list(rc *plugin.RequestContext) (any, error) {
	if _, err := rc.Page(); err != nil {
		return nil, err
	}
	items := []item{
		{Name: "alpha", Status: "ok"},
		{Name: "bravo", Status: "ok"},
		{Name: "charlie", Status: "degraded"},
	}
	return plugin.Page[item]{Items: items, NextCursor: ""}, nil
}

func (p *Plugin) echo(_ *plugin.RequestContext, client plugin.ClientStream) error {
	if _, err := client.Write([]byte("noop echo ready\n")); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			if _, werr := client.Write(buf[:n]); werr != nil {
				return nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return nil // client disconnected
		}
	}
}

type noopSession struct{}

func (s *noopSession) HealthCheck(context.Context) error { return nil }

func (s *noopSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *noopSession) Close() error { return nil }
