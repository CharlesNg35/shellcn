package swarm

import (
	"context"

	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// defaultSocket is the Docker Engine socket on a Swarm manager node.
const defaultSocket = "/var/run/docker.sock"

// Connect dials the Swarm manager's Docker daemon for this connection.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return dockerengine.Connect(ctx, cfg, defaultSocket)
}
