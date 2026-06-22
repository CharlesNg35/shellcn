package podman

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// defaultSocket is the Podman API socket the enrolled agent proxies.
const defaultSocket = "/run/podman/podman.sock"

// Connect dials Podman's Docker-compatible API socket for this connection.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return dockerengine.Connect(ctx, cfg, defaultSocket)
}
