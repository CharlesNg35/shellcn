package grpcplugin

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// GoPlugin bridges the ShellCN plugin contract to go-plugin's gRPC plugin. Impl
// is set on the serve (plugin) side only; the host uses GRPCClient.
type GoPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	Impl plugin.Plugin
}

func (g *GoPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterPluginServer(s, newServer(g.Impl, broker))
	return nil
}

func (g *GoPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (any, error) {
	return pluginv1.NewPluginClient(c), nil
}

// Plugins is the go-plugin set served and consumed under PluginName.
func Plugins(impl plugin.Plugin) goplugin.PluginSet {
	return goplugin.PluginSet{PluginName: &GoPlugin{Impl: impl}}
}
