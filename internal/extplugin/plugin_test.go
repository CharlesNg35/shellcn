package extplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func fixture() (plugin.Manifest, []plugin.Route) {
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "demo",
		Title:               "Demo",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{{
			Key: "data", Label: "Data", Type: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "demo.list"},
			Config: plugin.TableConfig{
				Columns: []plugin.Column{
					{Key: "id", Label: "ID"},
					{Key: "name", Label: "Name"},
				},
			},
		}},
	}
	routes := []plugin.Route{{
		ID: "demo.list", Method: plugin.MethodGet, Path: "/list",
		Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.list",
	}}
	return m, routes
}

type stubServer struct {
	pluginv1.UnimplementedPluginServer
	manifest []byte
}

func (s *stubServer) GetManifest(context.Context, *pluginv1.Empty) (*pluginv1.Manifest, error) {
	return &pluginv1.Manifest{Json: s.manifest}, nil
}

func (s *stubServer) Connect(context.Context, *pluginv1.ConnectRequest) (*pluginv1.SessionHandle, error) {
	return &pluginv1.SessionHandle{SessionId: "sess-1"}, nil
}

func (s *stubServer) Invoke(_ context.Context, req *pluginv1.InvokeRequest) (*pluginv1.InvokeResponse, error) {
	if req.GetParams()["fail"] == "1" {
		return nil, status.Error(codes.NotFound, "row gone")
	}
	if req.GetParams()["download"] == "1" {
		return &pluginv1.InvokeResponse{Download: &pluginv1.DownloadResponse{
			Name:   "app.wasm",
			Mime:   "application/wasm",
			Size:   4,
			Inline: true,
			Body:   []byte{0x00, 0x61, 0x73, 0x6d},
		}}, nil
	}
	out, _ := json.Marshal(map[string]any{
		"route":       req.GetRouteId(),
		"session":     req.GetSessionId(),
		"param":       req.GetParams()["k"],
		"proxyPrefix": req.GetProxyPrefix(),
	})
	return &pluginv1.InvokeResponse{ResultJson: out}, nil
}

func dialStub(t *testing.T, srv pluginv1.PluginServer) pluginv1.PluginClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	s := grpc.NewServer()
	pluginv1.RegisterPluginServer(s, srv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pluginv1.NewPluginClient(conn)
}

func TestProjectionMatchesInProcess(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, err := grpcplugin.EncodeManifest(manifest, routes)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	p, err := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	reg := pluginregistry.New()
	if err := reg.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, _ := reg.Projection("demo")
	want := plugin.BuildProjection(manifest, map[string]plugin.Route{routes[0].ID: routes[0]})
	if mustJSON(t, got) != mustJSON(t, want) {
		t.Fatalf("projection differs from in-process:\n got %s\nwant %s", mustJSON(t, got), mustJSON(t, want))
	}
}

func TestInvokeRoundTrip(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, _ := grpcplugin.EncodeManifest(manifest, routes)
	p, err := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	sess, err := p.Connect(ctx, plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect, Config: map[string]any{"host": "db"}})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	route := p.Routes()[0]

	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1"}, sess, map[string]string{"k": "v"}, nil, nil)
	res, err := route.Handle(rc)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	got := res.(map[string]any)
	if got["route"] != "demo.list" || got["session"] != "sess-1" || got["param"] != "v" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestInvokeErrorNormalized(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, _ := grpcplugin.EncodeManifest(manifest, routes)
	p, _ := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	sess, _ := p.Connect(ctx, plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})

	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1"}, sess, map[string]string{"fail": "1"}, nil, nil)
	_, err := p.Routes()[0].Handle(rc)
	if !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// borrowedHandle mimics internal/session.Handle: it implements plugin.Session
// and exposes the live session via Session(). The gateway dispatch always passes
// this wrapper as rc.Session, so the route shims must unwrap it.
type borrowedHandle struct{ inner plugin.Session }

func (h *borrowedHandle) Session() plugin.Session               { return h.inner }
func (h *borrowedHandle) HealthCheck(ctx context.Context) error { return h.inner.HealthCheck(ctx) }
func (h *borrowedHandle) Close() error                          { return h.inner.Close() }
func (h *borrowedHandle) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	return h.inner.OpenChannel(ctx, req)
}

// TestInvokeThroughSessionHandle is the regression test for routes failing with
// a bare "unavailable" when rc.Session is the core's borrowed handle rather
// than the raw grpc session.
func TestInvokeThroughSessionHandle(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, _ := grpcplugin.EncodeManifest(manifest, routes)
	p, err := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	sess, err := p.Connect(ctx, plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	route := p.Routes()[0]

	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1"}, &borrowedHandle{inner: sess}, map[string]string{"k": "v"}, nil, nil)
	res, err := route.Handle(rc)
	if err != nil {
		t.Fatalf("handle through borrowed handle: %v", err)
	}
	if got := res.(map[string]any); got["route"] != "demo.list" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestInvokeDownloadRoundTrip(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, _ := grpcplugin.EncodeManifest(manifest, routes)
	p, err := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	sess, err := p.Connect(ctx, plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1"}, sess, map[string]string{"download": "1"}, nil, nil)
	res, err := p.Routes()[0].Handle(rc)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	dl, ok := res.(*plugin.Download)
	if !ok {
		t.Fatalf("result = %T, want *plugin.Download", res)
	}
	body, err := io.ReadAll(dl.Body)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if dl.Name != "app.wasm" || dl.MIME != "application/wasm" || !dl.Inline || string(body) != "\x00asm" {
		t.Fatalf("download mismatch: %#v body=%#v", dl, body)
	}
}

// TestInvokeForwardsProxyPrefix asserts the proxy mount crosses the wire.
func TestInvokeForwardsProxyPrefix(t *testing.T) {
	ctx := context.Background()
	manifest, routes := fixture()
	bundle, _ := grpcplugin.EncodeManifest(manifest, routes)
	p, err := extplugin.New(ctx, dialStub(t, &stubServer{manifest: bundle}))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	sess, err := p.Connect(ctx, plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := plugin.NewRequestContext(ctx, plugin.User{ID: "u1"}, sess, nil, nil, nil).
		WithProxyPrefix("/api/connections/c1/proxy")
	res, err := p.Routes()[0].Handle(rc)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got := res.(map[string]any)["proxyPrefix"]; got != "/api/connections/c1/proxy" {
		t.Fatalf("proxy prefix across the wire = %#v", got)
	}
}
