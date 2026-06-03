package extplugin

import (
	"context"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// hostServer is the per-connection Host service the plugin calls back into.
// Network egress runs through the connection's transport (direct or agent), so
// the gateway stays the single egress point.
type hostServer struct {
	pluginv1.UnimplementedHostServer
	transport plugin.NetTransport
	broker    *goplugin.GRPCBroker
}

func newHostServer(transport plugin.NetTransport, broker *goplugin.GRPCBroker) *hostServer {
	return &hostServer{transport: transport, broker: broker}
}

func (h *hostServer) DialTarget(ctx context.Context, req *pluginv1.DialRequest) (*pluginv1.BrokerRef, error) {
	conn, err := h.transport.DialContext(ctx, req.GetNetwork(), req.GetAddress())
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	id := h.broker.NextId()
	go h.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		pluginv1.RegisterConnServer(s, grpcplugin.NewConnBridge(conn))
		return s
	})
	return &pluginv1.BrokerRef{BrokerId: id}, nil
}

func (h *hostServer) HTTPProxyEndpoint(context.Context, *pluginv1.SessionHandle) (*pluginv1.ProxyEndpoint, error) {
	base, _, ok := h.transport.HTTP()
	if !ok {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	return &pluginv1.ProxyEndpoint{BaseUrl: base}, nil
}
