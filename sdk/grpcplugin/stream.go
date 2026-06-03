package grpcplugin

import (
	"context"
	"net"

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
			WithAuditHook(cs.auditHook(req.GetSessionId()))
		return route.Stream(rc, &clientStream{conn: conn, ctx: ctx})
	})
	return &pluginv1.BrokerRef{BrokerId: ServeConn(s.broker, srv)}, nil
}

// OpenChannel opens a tracked upstream channel and bridges it to a brokered conn.
func (s *server) OpenChannel(ctx context.Context, req *pluginv1.ChannelRequest) (*pluginv1.BrokerRef, error) {
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
	srv := NewPipeServer(func(_ context.Context, conn net.Conn) error {
		Bridge(ch, conn)
		return nil
	})
	return &pluginv1.BrokerRef{BrokerId: ServeConn(s.broker, srv)}, nil
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
