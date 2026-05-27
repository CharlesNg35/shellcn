package docker

import (
	"context"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/dockerengine"
)

// defaultSocket is the standard Docker Engine unix socket used when a direct
// connection doesn't specify one.
const defaultSocket = "/var/run/docker.sock"

// Connect dials the Docker daemon for this connection's transport.
func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return dockerengine.Connect(ctx, cfg, defaultSocket)
}
