package grpcplugin

import (
	"context"
	"net"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ServeHTTPProxy serves the session's HTTPProxy over a brokered conn: the host
// bridges the browser's hijacked connection to it, so the plugin's reverse proxy
// (incl. WebSocket upgrades) reaches its upstream through the gateway.
func (s *server) ServeHTTPProxy(_ context.Context, req *pluginv1.ProxyRequest) (*pluginv1.BrokerRef, error) {
	cs := s.conn(req.GetSessionId())
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	proxier, ok := cs.session.(plugin.HTTPProxy)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "session has no HTTP proxy")
	}
	srv := NewPipeServer(func(_ context.Context, conn net.Conn) error {
		return http.Serve(newSingleConnListener(conn), http.HandlerFunc(proxier.ServeHTTPProxy))
	})
	return &pluginv1.BrokerRef{BrokerId: ServeConn(s.broker, srv)}, nil
}
