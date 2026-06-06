package extplugin

import (
	"context"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
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
	storage   plugin.Storage
	broker    *goplugin.GRPCBroker
	audit     AuditFunc
}

func newHostServer(transport plugin.NetTransport, storage plugin.Storage, broker *goplugin.GRPCBroker, audit AuditFunc) *hostServer {
	return &hostServer{transport: transport, storage: storage, broker: broker, audit: audit}
}

func (h *hostServer) DialTarget(ctx context.Context, req *pluginv1.DialRequest) (*pluginv1.BrokerRef, error) {
	if h.transport == nil {
		return nil, status.Error(codes.Unavailable, "connection has no transport")
	}
	conn, err := h.transport.DialContext(ctx, req.GetNetwork(), req.GetAddress())
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return &pluginv1.BrokerRef{BrokerId: grpcplugin.ServeConn(h.broker, grpcplugin.NewConnBridge(conn))}, nil
}

func (h *hostServer) HTTPProxyEndpoint(context.Context, *pluginv1.SessionHandle) (*pluginv1.ProxyEndpoint, error) {
	if h.transport == nil {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	base, _, ok := h.transport.HTTP()
	if !ok {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	return &pluginv1.ProxyEndpoint{BaseUrl: base}, nil
}

func (h *hostServer) OpenHTTPConn(context.Context, *pluginv1.SessionHandle) (*pluginv1.BrokerRef, error) {
	if h.transport == nil {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	base, rt, ok := h.transport.HTTP()
	if !ok {
		return nil, status.Error(codes.Unavailable, "connection has no L7 transport")
	}
	bridge, err := grpcplugin.NewHTTPProxyBridge(base, rt)
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return &pluginv1.BrokerRef{BrokerId: grpcplugin.ServeConn(h.broker, bridge)}, nil
}

func (h *hostServer) Audit(_ context.Context, rec *pluginv1.AuditRecord) (*pluginv1.Empty, error) {
	if h.audit != nil {
		h.audit(plugin.AuditResult(rec.GetResult()), rec.GetParams(), rec.GetError())
	}
	return &pluginv1.Empty{}, nil
}

func (h *hostServer) StorageGet(ctx context.Context, req *pluginv1.StorageGetRequest) (*pluginv1.StorageItem, error) {
	if h.storage == nil {
		return nil, status.Error(codes.Unavailable, "plugin storage unavailable")
	}
	item, err := h.storage.Get(ctx, pluginStorageScope(req.GetScope()), req.GetKey())
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return wireStorageItem(item), nil
}

func (h *hostServer) StoragePut(ctx context.Context, req *pluginv1.StorageItem) (*pluginv1.StorageItem, error) {
	if h.storage == nil {
		return nil, status.Error(codes.Unavailable, "plugin storage unavailable")
	}
	item, err := h.storage.Put(ctx, pluginStorageItem(req))
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return wireStorageItem(item), nil
}

func (h *hostServer) StorageDelete(ctx context.Context, req *pluginv1.StorageDeleteRequest) (*pluginv1.Empty, error) {
	if h.storage == nil {
		return nil, status.Error(codes.Unavailable, "plugin storage unavailable")
	}
	if err := h.storage.Delete(ctx, pluginStorageScope(req.GetScope()), req.GetKey()); err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	return &pluginv1.Empty{}, nil
}

func (h *hostServer) StorageList(ctx context.Context, req *pluginv1.StorageListRequest) (*pluginv1.StorageListResponse, error) {
	if h.storage == nil {
		return nil, status.Error(codes.Unavailable, "plugin storage unavailable")
	}
	items, err := h.storage.List(ctx, pluginStorageScope(req.GetScope()), req.GetPrefix())
	if err != nil {
		return nil, grpcplugin.StatusFromError(err)
	}
	out := make([]*pluginv1.StorageItem, len(items))
	for i, item := range items {
		out[i] = wireStorageItem(item)
	}
	return &pluginv1.StorageListResponse{Items: out}, nil
}

func pluginStorageScope(scope *pluginv1.StorageScope) plugin.StorageScope {
	if scope == nil {
		return plugin.StorageScope{}
	}
	return plugin.StorageScope{
		Namespace:    scope.GetNamespace(),
		Plugin:       scope.GetPlugin(),
		Protocol:     scope.GetProtocol(),
		ConnectionID: scope.GetConnectionId(),
		OwnerID:      scope.GetOwnerId(),
		Shared:       scope.GetShared(),
	}
}

func wireStorageScope(scope plugin.StorageScope) *pluginv1.StorageScope {
	return &pluginv1.StorageScope{
		Namespace:    scope.Namespace,
		Plugin:       scope.Plugin,
		Protocol:     scope.Protocol,
		ConnectionId: scope.ConnectionID,
		OwnerId:      scope.OwnerID,
		Shared:       scope.Shared,
	}
}

func pluginStorageItem(item *pluginv1.StorageItem) plugin.StorageItem {
	if item == nil {
		return plugin.StorageItem{}
	}
	return plugin.StorageItem{
		Scope:       pluginStorageScope(item.GetScope()),
		Key:         item.GetKey(),
		Value:       append([]byte(nil), item.GetValue()...),
		ContentType: item.GetContentType(),
		Metadata:    cloneStringMap(item.GetMetadata()),
		CreatedAt:   unixNanoTime(item.GetCreatedAtUnixNano()),
		UpdatedAt:   unixNanoTime(item.GetUpdatedAtUnixNano()),
	}
}

func wireStorageItem(item plugin.StorageItem) *pluginv1.StorageItem {
	return &pluginv1.StorageItem{
		Scope:             wireStorageScope(item.Scope),
		Key:               item.Key,
		Value:             append([]byte(nil), item.Value...),
		ContentType:       item.ContentType,
		Metadata:          cloneStringMap(item.Metadata),
		CreatedAtUnixNano: item.CreatedAt.UnixNano(),
		UpdatedAtUnixNano: item.UpdatedAt.UnixNano(),
	}
}

func unixNanoTime(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
