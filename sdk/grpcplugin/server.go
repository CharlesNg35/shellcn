package grpcplugin

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// server is the plugin-side implementation of the Plugin service. It holds the
// live sessions keyed by the opaque id handed back to the host at Connect.
type server struct {
	pluginv1.UnimplementedPluginServer
	impl   plugin.Plugin
	broker *goplugin.GRPCBroker
	routes map[string]plugin.Route

	mu       sync.Mutex
	sessions map[string]plugin.Session
	seq      atomic.Uint64
}

func newServer(impl plugin.Plugin, broker *goplugin.GRPCBroker) *server {
	routes := make(map[string]plugin.Route)
	for _, r := range impl.Routes() {
		routes[r.ID] = r
	}
	return &server{impl: impl, broker: broker, routes: routes, sessions: make(map[string]plugin.Session)}
}

func (s *server) GetManifest(context.Context, *pluginv1.Empty) (*pluginv1.Manifest, error) {
	data, err := EncodeManifest(s.impl.Manifest(), s.impl.Routes())
	if err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.Manifest{Json: data}, nil
}

func (s *server) Connect(ctx context.Context, req *pluginv1.ConnectRequest) (*pluginv1.SessionHandle, error) {
	cfg := plugin.ConnectConfig{ConnectionID: req.GetConnectionId(), Transport: plugin.Transport(req.GetTransport())}
	if raw := req.GetConfigJson(); len(raw) > 0 {
		if err := json.Unmarshal(raw, &cfg.Config); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	if id := req.GetHostBrokerId(); id != 0 && s.broker != nil {
		cc, err := s.broker.Dial(id)
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		cfg.Net = newBrokerTransport(s.broker, pluginv1.NewHostClient(cc))
	}
	sess, err := s.impl.Connect(ctx, cfg)
	if err != nil {
		return nil, StatusFromError(err)
	}
	id := strconv.FormatUint(s.seq.Add(1), 10)
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return &pluginv1.SessionHandle{SessionId: id}, nil
}

func (s *server) HealthCheck(ctx context.Context, h *pluginv1.SessionHandle) (*pluginv1.Empty, error) {
	sess := s.session(h.GetSessionId())
	if sess == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	if err := sess.HealthCheck(ctx); err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *server) Close(_ context.Context, h *pluginv1.SessionHandle) (*pluginv1.Empty, error) {
	id := h.GetSessionId()
	s.mu.Lock()
	sess := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if sess != nil {
		_ = sess.Close()
	}
	return &pluginv1.Empty{}, nil
}

func (s *server) Invoke(ctx context.Context, req *pluginv1.InvokeRequest) (*pluginv1.InvokeResponse, error) {
	sess := s.session(req.GetSessionId())
	if sess == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	route, ok := s.routes[req.GetRouteId()]
	if !ok || route.Handle == nil {
		return nil, status.Error(codes.NotFound, "unknown route")
	}
	rc := plugin.NewRequestContext(ctx, actingUser(req.GetUser()), sess, req.GetParams(), queryValues(req.GetQuery()), req.GetBody())
	res, err := route.Handle(rc)
	if err != nil {
		return nil, StatusFromError(err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.InvokeResponse{ResultJson: data}, nil
}

func (s *server) session(id string) plugin.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

func actingUser(u *pluginv1.ActingUser) plugin.User {
	if u == nil {
		return plugin.User{}
	}
	return plugin.User{ID: u.GetId(), Username: u.GetUsername(), DisplayName: u.GetDisplayName(), Roles: u.GetRoles()}
}

func queryValues(q map[string]string) url.Values {
	if len(q) == 0 {
		return nil
	}
	v := make(url.Values, len(q))
	for k, val := range q {
		v.Set(k, val)
	}
	return v
}
