// Command demoplugin is a minimal out-of-tree plugin used by the extplugin
// end-to-end test.
package main

import (
	"context"
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
			ID: "demo.crash", Method: plugin.MethodPost, Path: "/crash",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.crash",
			Handle: func(*plugin.RequestContext) (any, error) { os.Exit(1); return nil, nil },
		},
	}
}

func (demo) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return demoSession{}, nil
}

type demoSession struct{}

func (demoSession) HealthCheck(context.Context) error { return nil }

func (demoSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (demoSession) Close() error { return nil }

func main() { sdk.Serve(demo{}) }
