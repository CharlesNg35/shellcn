package swarm

import (
	"context"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/shared/dockerengine"
)

// defaultSocket is the Docker Engine socket on a Swarm manager node.
const defaultSocket = "/var/run/docker.sock"

// Connect dials the Swarm manager's Docker daemon for this connection.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return dockerengine.Connect(ctx, cfg, defaultSocket)
}
