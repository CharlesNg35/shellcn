package grpcplugin

import (
	"context"
	"net"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// OpenStream runs a streaming route's handler over a brokered conn presented to
// it as a ClientStream; the host bridges that conn to the browser.
func (s *server) OpenStream(_ context.Context, req *pluginv1.StreamStart) (*pluginv1.BrokerRef, error) {
	cs := s.conn(req.GetSessionId())
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	route, ok := s.routes[req.GetRouteId()]
	if !ok || route.Stream == nil {
		return nil, status.Error(codes.NotFound, "unknown stream route")
	}
	srv := NewPipeServer(func(ctx context.Context, conn net.Conn) error {
		rc := plugin.NewRequestContext(ctx, actingUser(req.GetUser()), cs.session, req.GetParams(), nil, nil).
			WithStorage(newHostStorage(cs.host)).
			WithAuditHook(cs.auditHook(req.GetSessionId())).
			WithProxyPrefix(req.GetProxyPrefix())
		return route.Stream(rc, &clientStream{conn: conn, ctx: ctx})
	})
	return &pluginv1.BrokerRef{BrokerId: ServeConn(s.broker, srv)}, nil
}

// resizableChannel and serverInitChannel are the optional Channel capabilities
// the wire re-exposes, mirroring the core's tracked-channel detection.
type (
	resizableChannel  interface{ Resize(cols, rows int) error }
	serverInitChannel interface{ ServerInit() []byte }
)

// OpenChannel opens a tracked upstream channel and bridges it to a brokered
// conn, declaring the channel's optional capabilities so the host-side wrapper
// keeps parity with an in-process Channel.
func (s *server) OpenChannel(ctx context.Context, req *pluginv1.ChannelRequest) (*pluginv1.ChannelInfo, error) {
	cs := s.conn(req.GetSessionId())
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	ch, err := cs.session.OpenChannel(ctx, plugin.ChannelRequest{
		Kind:   plugin.StreamKind(req.GetKind()),
		Params: req.GetParams(),
	})
	if err != nil {
		return nil, StatusFromError(err)
	}
	info := &pluginv1.ChannelInfo{
		ChannelId: strconv.FormatUint(s.chanSeq.Add(1), 10),
	}
	if _, ok := ch.(resizableChannel); ok {
		info.Resizable = true
	}
	if si, ok := ch.(serverInitChannel); ok {
		info.ServerInit = si.ServerInit()
	}
	cs.trackChannel(info.ChannelId, ch)
	srv := NewPipeServer(func(_ context.Context, conn net.Conn) error {
		defer cs.untrackChannel(info.ChannelId)
		Bridge(ch, conn)
		return nil
	})
	info.BrokerId = ServeConn(s.broker, srv)
	return info, nil
}

// ResizeChannel forwards a terminal resize to an open, resizable channel.
func (s *server) ResizeChannel(_ context.Context, req *pluginv1.ChannelResize) (*pluginv1.Empty, error) {
	cs := s.conn(req.GetSessionId())
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	ch := cs.channel(req.GetChannelId())
	if ch == nil {
		return nil, status.Error(codes.NotFound, "unknown channel")
	}
	resizer, ok := ch.(resizableChannel)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "channel is not resizable")
	}
	if err := resizer.Resize(int(req.GetCols()), int(req.GetRows())); err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.Empty{}, nil
}

// clientStream presents a brokered conn as a plugin.ClientStream.
type clientStream struct {
	conn net.Conn
	ctx  context.Context
}

func (c *clientStream) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *clientStream) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *clientStream) Close() error                { return c.conn.Close() }
func (c *clientStream) Context() context.Context    { return c.ctx }
