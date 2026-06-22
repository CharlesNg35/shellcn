package docker

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// defaultSocket is the Docker Engine unix socket the enrolled agent proxies.
const defaultSocket = "/var/run/docker.sock"

// Connect dials the Docker daemon for this connection's transport.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return dockerengine.Connect(ctx, cfg, defaultSocket)
}
