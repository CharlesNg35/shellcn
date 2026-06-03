// Command demoplugin is a minimal out-of-tree plugin used by the extplugin
// end-to-end tests.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/charlesng35/shellcn/sdk"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type demo struct{}

func (demo) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "demo",
		Title:               "Demo",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{{
			Key: "data", Label: "Data", Type: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "demo.list"},
			Config: plugin.TableConfig{Editable: true, RowKey: []string{"id"}},
		}},
	}
}

func (demo) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "demo.list", Method: plugin.MethodGet, Path: "/list",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.list",
			Handle: func(rc *plugin.RequestContext) (any, error) {
				return map[string]any{"items": []string{"alpha", rc.Param("k")}}, nil
			},
		},
		{
			ID: "demo.echo", Method: plugin.MethodPost, Path: "/echo",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.echo",
			Handle: echo,
		},
		{
			ID: "demo.crash", Method: plugin.MethodPost, Path: "/crash",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.crash",
			Handle: func(*plugin.RequestContext) (any, error) { os.Exit(1); return nil, nil },
		},
	}
}

// echo dials the configured target through the core-provided transport and
// returns its echo, exercising brokered egress.
func echo(rc *plugin.RequestContext) (any, error) {
	s := rc.Session.(*demoSession)
	if s.transport == nil {
		return nil, fmt.Errorf("%w: no transport", plugin.ErrUnavailable)
	}
	conn, err := s.transport.DialContext(rc.Ctx, "tcp", s.target)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write(rc.Body()); err != nil {
		return nil, err
	}
	buf := make([]byte, len(rc.Body()))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return map[string]any{"echo": string(buf)}, nil
}

func (demo) Connect(_ context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return &demoSession{transport: cfg.Net, target: cfg.String("target")}, nil
}

type demoSession struct {
	transport plugin.NetTransport
	target    string
}

func (demoSession) HealthCheck(context.Context) error { return nil }

func (demoSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (demoSession) Close() error { return nil }

func main() { sdk.Serve(demo{}) }
