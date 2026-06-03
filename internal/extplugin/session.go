package extplugin

import (
	"context"
	"net"

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

func (s *grpcSession) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	client, broker := s.ref.get()
	ref, err := client.OpenChannel(ctx, &pluginv1.ChannelRequest{
		SessionId: s.id, Kind: string(req.Kind), Params: req.Params,
	})
	if err != nil {
		return nil, grpcplugin.ErrorFromStatus(err)
	}
	conn, err := grpcplugin.DialConn(broker, ref.GetBrokerId())
	if err != nil {
		return nil, err
	}
	return &grpcChannel{conn: conn, kind: req.Kind}, nil
}

type grpcChannel struct {
	conn net.Conn
	kind plugin.StreamKind
}

func (c *grpcChannel) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *grpcChannel) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *grpcChannel) Close() error                { return c.conn.Close() }
func (c *grpcChannel) Kind() plugin.StreamKind     { return c.kind }

func (s *grpcSession) Close() error {
	client, _ := s.ref.get()
	_, err := client.Close(context.Background(), &pluginv1.SessionHandle{SessionId: s.id})
	return grpcplugin.ErrorFromStatus(err)
}
