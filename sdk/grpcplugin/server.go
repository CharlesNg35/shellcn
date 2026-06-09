package grpcplugin

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// connState is one live session plus the Host client used to reach the core for
// egress and audit, and the open channels addressable by control RPCs (resize).
type connState struct {
	session plugin.Session
	host    pluginv1.HostClient

	chanMu   sync.Mutex
	channels map[string]plugin.Channel
}

func (cs *connState) trackChannel(id string, ch plugin.Channel) {
	cs.chanMu.Lock()
	if cs.channels == nil {
		cs.channels = make(map[string]plugin.Channel)
	}
	cs.channels[id] = ch
	cs.chanMu.Unlock()
}

func (cs *connState) untrackChannel(id string) {
	cs.chanMu.Lock()
	delete(cs.channels, id)
	cs.chanMu.Unlock()
}

func (cs *connState) channel(id string) plugin.Channel {
	cs.chanMu.Lock()
	defer cs.chanMu.Unlock()
	return cs.channels[id]
}

// server is the plugin-side implementation of the Plugin service. It holds the
// live sessions keyed by the opaque id handed back to the host at Connect.
type server struct {
	pluginv1.UnimplementedPluginServer
	impl   plugin.Plugin
	broker *goplugin.GRPCBroker
	routes map[string]plugin.Route

	mu       sync.Mutex
	sessions map[string]*connState
	seq      atomic.Uint64
	chanSeq  atomic.Uint64
}

func newServer(impl plugin.Plugin, broker *goplugin.GRPCBroker) *server {
	routes := make(map[string]plugin.Route)
	for _, r := range impl.Routes() {
		routes[r.ID] = r
	}
	return &server{impl: impl, broker: broker, routes: routes, sessions: make(map[string]*connState)}
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
	var host pluginv1.HostClient
	if id := req.GetHostBrokerId(); id != 0 && s.broker != nil {
		cc, err := s.broker.Dial(id)
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		host = pluginv1.NewHostClient(cc)
		cfg.Net = newBrokerTransport(s.broker, host)
		cfg.Storage = newHostStorage(host)
	}
	sess, err := s.impl.Connect(ctx, cfg)
	if err != nil {
		return nil, StatusFromError(err)
	}
	id := strconv.FormatUint(s.seq.Add(1), 10)
	s.mu.Lock()
	s.sessions[id] = &connState{session: sess, host: host}
	s.mu.Unlock()
	return &pluginv1.SessionHandle{SessionId: id}, nil
}

func (s *server) HealthCheck(ctx context.Context, h *pluginv1.SessionHandle) (*pluginv1.Empty, error) {
	cs := s.conn(h.GetSessionId())
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	if err := cs.session.HealthCheck(ctx); err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *server) Close(_ context.Context, h *pluginv1.SessionHandle) (*pluginv1.Empty, error) {
	id := h.GetSessionId()
	s.mu.Lock()
	cs := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if cs != nil {
		_ = cs.session.Close()
	}
	return &pluginv1.Empty{}, nil
}

func (s *server) Invoke(ctx context.Context, req *pluginv1.InvokeRequest) (*pluginv1.InvokeResponse, error) {
	id := req.GetSessionId()
	cs := s.conn(id)
	if cs == nil {
		return nil, status.Error(codes.NotFound, "unknown session")
	}
	route, ok := s.routes[req.GetRouteId()]
	if !ok || route.Handle == nil {
		return nil, status.Error(codes.NotFound, "unknown route")
	}
	rc := plugin.NewRequestContext(ctx, actingUser(req.GetUser()), cs.session, req.GetParams(), queryValues(req.GetQuery()), req.GetBody()).
		WithStorage(newHostStorage(cs.host)).
		WithAuditHook(cs.auditHook(id)).
		WithProxyPrefix(req.GetProxyPrefix())
	res, err := route.Handle(rc)
	if err != nil {
		return nil, StatusFromError(err)
	}
	if dl, ok := res.(*plugin.Download); ok {
		out, err := encodeDownload(dl)
		if err != nil {
			return nil, StatusFromError(err)
		}
		return out, nil
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, StatusFromError(err)
	}
	return &pluginv1.InvokeResponse{ResultJson: data}, nil
}

func encodeDownload(dl *plugin.Download) (*pluginv1.InvokeResponse, error) {
	var r io.Reader
	switch {
	case dl.Body != nil:
		defer func() { _ = dl.Body.Close() }()
		r = dl.Body
	case dl.Seeker != nil:
		defer func() { _ = dl.Seeker.Close() }()
		if _, err := dl.Seeker.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		r = dl.Seeker
	case dl.OpenRange != nil:
		body, err := dl.OpenRange(0, -1)
		if err != nil {
			return nil, err
		}
		defer func() { _ = body.Close() }()
		r = body
	}
	if r == nil {
		return &pluginv1.InvokeResponse{Download: &pluginv1.DownloadResponse{
			Name:   dl.Name,
			Mime:   dl.MIME,
			Size:   dl.Size,
			Inline: dl.Inline,
		}}, nil
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	size := dl.Size
	if size < 0 {
		size = int64(len(body))
	}
	var modTime int64
	if !dl.ModTime.IsZero() {
		modTime = dl.ModTime.UnixNano()
	}
	return &pluginv1.InvokeResponse{Download: &pluginv1.DownloadResponse{
		Name:            dl.Name,
		Mime:            dl.MIME,
		Size:            size,
		ModTimeUnixNano: modTime,
		Inline:          dl.Inline,
		Body:            body,
	}}, nil
}

func (s *server) conn(id string) *connState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

// auditHook forwards a handler's stream-internal audit to the core via Host.Audit.
func (cs *connState) auditHook(sessionID string) plugin.AuditHook {
	if cs.host == nil {
		return nil
	}
	return func(ctx context.Context, result plugin.AuditResult, params map[string]string, err error) {
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_, _ = cs.host.Audit(ctx, &pluginv1.AuditRecord{
			SessionId: sessionID, Result: string(result), Params: params, Error: msg,
		})
	}
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
