package extplugin

import (
	"context"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type grpcSession struct {
	id  string
	ref *clientRef
}

func (s *grpcSession) HealthCheck(ctx context.Context) error {
	client, _ := s.ref.get()
	_, err := client.HealthCheck(ctx, &pluginv1.SessionHandle{SessionId: s.id})
	return grpcplugin.ErrorFromStatus(err)
}

func (s *grpcSession) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported // wired in Step 5
}

func (s *grpcSession) Close() error {
	client, _ := s.ref.get()
	_, err := client.Close(context.Background(), &pluginv1.SessionHandle{SessionId: s.id})
	return grpcplugin.ErrorFromStatus(err)
}
