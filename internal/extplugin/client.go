package extplugin

import (
	"sync"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
)

// clientRef holds the live gRPC client and broker for a plugin so the supervisor
// can swap them after a crash-respawn without re-registering the plugin.
type clientRef struct {
	mu     sync.RWMutex
	client pluginv1.PluginClient
	broker *goplugin.GRPCBroker
}

func (r *clientRef) get() (pluginv1.PluginClient, *goplugin.GRPCBroker) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client, r.broker
}

func (r *clientRef) set(c pluginv1.PluginClient, b *goplugin.GRPCBroker) {
	r.mu.Lock()
	r.client, r.broker = c, b
	r.mu.Unlock()
}
