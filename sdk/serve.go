// Package sdk is the entry point an out-of-tree ShellCN plugin imports: declare
// the contract with sdk/plugin, then call Serve from main.
package sdk

import (
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Serve runs a plugin as a go-plugin gRPC subprocess. It blocks until the host
// disconnects, and is the last call in a plugin's main.
func Serve(p plugin.Plugin) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: grpcplugin.Handshake,
		Plugins:         grpcplugin.Plugins(p),
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}
