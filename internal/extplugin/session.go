package extplugin

import (
	"context"
	"net"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
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
	info, err := client.OpenChannel(ctx, &pluginv1.ChannelRequest{
		SessionId: s.id, Kind: string(req.Kind), Params: req.Params,
	})
	if err != nil {
		return nil, grpcplugin.ErrorFromStatus(err)
	}
	conn, err := grpcplugin.DialConn(broker, info.GetBrokerId())
	if err != nil {
		return nil, err
	}
	// Re-expose the channel's declared capabilities so the core's tracked-channel
	// wrapper detects them exactly as for an in-process Channel.
	base := &grpcChannel{conn: conn, kind: req.Kind}
	resizable := info.GetResizable()
	serverInit := info.GetServerInit()
	switch {
	case resizable && len(serverInit) > 0:
		return &grpcResizableDesktopChannel{
			grpcResizableChannel: grpcResizableChannel{grpcChannel: base, sess: s, channelID: info.GetChannelId()},
			serverInit:           serverInit,
		}, nil
	case resizable:
		return &grpcResizableChannel{grpcChannel: base, sess: s, channelID: info.GetChannelId()}, nil
	case len(serverInit) > 0:
		return &grpcDesktopChannel{grpcChannel: base, serverInit: serverInit}, nil
	default:
		return base, nil
	}
}

type grpcChannel struct {
	conn net.Conn
	kind plugin.StreamKind
}

func (c *grpcChannel) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *grpcChannel) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *grpcChannel) Close() error                { return c.conn.Close() }
func (c *grpcChannel) Kind() plugin.StreamKind     { return c.kind }

type grpcResizableChannel struct {
	*grpcChannel
	sess      *grpcSession
	channelID string
}

func (c *grpcResizableChannel) Resize(cols, rows int) error {
	client, _ := c.sess.ref.get()
	_, err := client.ResizeChannel(context.Background(), &pluginv1.ChannelResize{
		SessionId: c.sess.id, ChannelId: c.channelID,
		Cols: int32(cols), Rows: int32(rows),
	})
	return grpcplugin.ErrorFromStatus(err)
}

type grpcDesktopChannel struct {
	*grpcChannel
	serverInit []byte
}

func (c *grpcDesktopChannel) ServerInit() []byte { return c.serverInit }

type grpcResizableDesktopChannel struct {
	grpcResizableChannel
	serverInit []byte
}

func (c *grpcResizableDesktopChannel) ServerInit() []byte { return c.serverInit }

func (s *grpcSession) Close() error {
	client, _ := s.ref.get()
	_, err := client.Close(context.Background(), &pluginv1.SessionHandle{SessionId: s.id})
	return grpcplugin.ErrorFromStatus(err)
}
