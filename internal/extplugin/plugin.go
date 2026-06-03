// Package extplugin adapts an out-of-process plugin subprocess to plugin.Plugin
// so the registry, projection, and route wrapper treat it like a built-in.
package extplugin

import (
	"context"
	"encoding/json"
	"net/url"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type grpcPlugin struct {
	ref      *clientRef
	manifest plugin.Manifest
	routes   []plugin.Route
}

// New fetches and reconstructs the manifest once, binding each route to a gRPC
// shim that forwards to the subprocess.
func New(ctx context.Context, client pluginv1.PluginClient) (plugin.Plugin, error) {
	return newPlugin(ctx, &clientRef{client: client})
}

func newPlugin(ctx context.Context, ref *clientRef) (plugin.Plugin, error) {
	resp, err := ref.get().GetManifest(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, grpcplugin.ErrorFromStatus(err)
	}
	manifest, routes, err := grpcplugin.DecodeManifest(resp.GetJson())
	if err != nil {
		return nil, err
	}
	g := &grpcPlugin{ref: ref, manifest: manifest, routes: routes}
	for i := range g.routes {
		g.bind(&g.routes[i])
	}
	return g, nil
}

func (g *grpcPlugin) Manifest() plugin.Manifest { return g.manifest }
func (g *grpcPlugin) Routes() []plugin.Route    { return g.routes }

func (g *grpcPlugin) Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	config, err := json.Marshal(cfg.Config)
	if err != nil {
		return nil, err
	}
	resp, err := g.ref.get().Connect(ctx, &pluginv1.ConnectRequest{
		ConnectionId: cfg.ConnectionID,
		Transport:    string(cfg.Transport),
		ConfigJson:   config,
	})
	if err != nil {
		return nil, grpcplugin.ErrorFromStatus(err)
	}
	return &grpcSession{id: resp.GetSessionId(), ref: g.ref}, nil
}

func (g *grpcPlugin) bind(r *plugin.Route) {
	if r.IsStream() {
		r.Stream = unsupportedStream // wired in Step 5
		return
	}
	routeID := r.ID
	r.Handle = func(rc *plugin.RequestContext) (any, error) {
		return g.invoke(rc, routeID)
	}
}

func (g *grpcPlugin) invoke(rc *plugin.RequestContext, routeID string) (any, error) {
	sess, ok := rc.Session.(*grpcSession)
	if !ok {
		return nil, plugin.ErrUnavailable
	}
	resp, err := g.ref.get().Invoke(rc.Ctx, &pluginv1.InvokeRequest{
		SessionId: sess.id,
		RouteId:   routeID,
		Params:    rc.Params(),
		Query:     flattenQuery(rc.Query()),
		Body:      rc.Body(),
		User:      wireUser(rc.User),
	})
	if err != nil {
		return nil, grpcplugin.ErrorFromStatus(err)
	}
	if len(resp.GetResultJson()) == 0 {
		return nil, nil
	}
	var result any
	if err := json.Unmarshal(resp.GetResultJson(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func unsupportedStream(*plugin.RequestContext, plugin.ClientStream) error {
	return plugin.ErrNotSupported
}

func wireUser(u plugin.User) *pluginv1.ActingUser {
	return &pluginv1.ActingUser{Id: u.ID, Username: u.Username, DisplayName: u.DisplayName, Roles: u.Roles}
}

func flattenQuery(q url.Values) map[string]string {
	if len(q) == 0 {
		return nil
	}
	out := make(map[string]string, len(q))
	for k := range q {
		out[k] = q.Get(k)
	}
	return out
}
