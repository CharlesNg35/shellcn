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

// AuditFunc records a stream-internal operation a plugin reported via Host.Audit.
type AuditFunc func(result plugin.AuditResult, params map[string]string, errMsg string)

// hostServer is the per-connection Host service the plugin calls back into.
// Network egress runs through the connection's transport (direct or agent) and
// audit forwards to the core, so the gateway stays the single egress + audit point.
type hostServer struct {
	pluginv1.UnimplementedHostServer
	transport plugin.NetTransport
	broker    *goplugin.GRPCBroker
	audit     AuditFunc
}

func newHostServer(transport plugin.NetTransport, broker *goplugin.GRPCBroker, audit AuditFunc) *hostServer {
	return &hostServer{transport: transport, broker: broker, audit: audit}
}

func (h *hostServer) DialTarget(ctx context.Context, req *pluginv1.DialRequest) (*pluginv1.BrokerRef, error) {
	conn, err := h.transport.DialContext(ctx, req.GetNetwork(), req.GetAddress())
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return h.serveConn(grpcplugin.NewConnBridge(conn)), nil
}

func (h *hostServer) HTTPProxyEndpoint(context.Context, *pluginv1.SessionHandle) (*pluginv1.ProxyEndpoint, error) {
	base, _, ok := h.transport.HTTP()
	if !ok {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	return &pluginv1.ProxyEndpoint{BaseUrl: base}, nil
}

func (h *hostServer) OpenHTTPConn(context.Context, *pluginv1.SessionHandle) (*pluginv1.BrokerRef, error) {
	base, rt, ok := h.transport.HTTP()
	if !ok {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	bridge, err := grpcplugin.NewHTTPProxyBridge(base, rt)
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return h.serveConn(bridge), nil
}

func (h *hostServer) Audit(_ context.Context, rec *pluginv1.AuditRecord) (*pluginv1.Empty, error) {
	if h.audit != nil {
		h.audit(plugin.AuditResult(rec.GetResult()), rec.GetParams(), rec.GetError())
	}
	return &pluginv1.Empty{}, nil
}

func (h *hostServer) serveConn(srv pluginv1.ConnServer) *pluginv1.BrokerRef {
	id := h.broker.NextId()
	go h.broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		pluginv1.RegisterConnServer(s, srv)
		return s
	})
	return &pluginv1.BrokerRef{BrokerId: id}
}
