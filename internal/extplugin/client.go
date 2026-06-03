package extplugin

import (
	"sync"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
)

// clientRef holds the live gRPC client for a plugin so the supervisor can swap
// it after a crash-respawn without re-registering the plugin.
type clientRef struct {
	mu     sync.RWMutex
	client pluginv1.PluginClient
}

func (r *clientRef) get() pluginv1.PluginClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

func (r *clientRef) set(c pluginv1.PluginClient) {
	r.mu.Lock()
	r.client = c
	r.mu.Unlock()
}
