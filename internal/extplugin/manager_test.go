package extplugin_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func echoServer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(conn, conn); _ = conn.Close() }()
		}
	}()
	return lis.Addr().String()
}

func buildDemo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "demoplugin")
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/demoplugin").CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}
	return dir
}

func routeByID(p plugin.Plugin, id string) plugin.Route {
	for _, r := range p.Routes() {
		if r.ID == id {
			return r
		}
	}
	return plugin.Route{}
}

func TestManagerLoadsSubprocessPlugin(t *testing.T) {
	dir := buildDemo(t)

	reg := pluginregistry.New()
	m := extplugin.NewManager(dir)
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}

	p, ok := reg.Get("demo")
	if !ok {
		t.Fatal("demo plugin not registered")
	}

	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if err := sess.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}

	var route plugin.Route
	for _, r := range p.Routes() {
		if r.ID == "demo.list" {
			route = r
		}
	}
	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, map[string]string{"k": "v"}, nil, nil)
	res, err := route.Handle(rc)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	items := res.(map[string]any)["items"].([]any)
	if len(items) != 2 || items[0] != "alpha" || items[1] != "v" {
		t.Fatalf("unexpected result over subprocess: %v", res)
	}
}

func TestManagerRespawnsCrashedPlugin(t *testing.T) {
	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")

	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Crash the subprocess; the in-flight call errors and the session dies.
	_, _ = routeByID(p, "demo.crash").Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, nil))

	// The supervisor should respawn the binary so a fresh Connect+Invoke works.
	deadline := time.Now().Add(15 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("plugin did not recover after crash")
		}
		s, err := p.Connect(context.Background(), plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
		if err == nil {
			res, herr := routeByID(p, "demo.list").Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, s, nil, nil, nil))
			_ = s.Close()
			if herr == nil && res != nil {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func TestPluginEgressThroughCore(t *testing.T) {
	target := echoServer(t)
	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")

	// With a core transport, the plugin reaches the target through the gateway.
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect,
		Config: map[string]any{"target": target},
		Net:    plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	rc := plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, []byte("ping"))
	res, err := routeByID(p, "demo.echo").Handle(rc)
	if err != nil {
		t.Fatalf("echo through core: %v", err)
	}
	if res.(map[string]any)["echo"] != "ping" {
		t.Fatalf("unexpected echo: %v", res)
	}

	// Without a core transport, egress is impossible — the plugin can't dial out.
	noNet, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c2", Transport: plugin.TransportDirect,
		Config: map[string]any{"target": target},
	})
	if err != nil {
		t.Fatalf("connect (no net): %v", err)
	}
	defer func() { _ = noNet.Close() }()
	rc2 := plugin.NewRequestContext(context.Background(), plugin.User{}, noNet, nil, nil, []byte("ping"))
	if _, err := routeByID(p, "demo.echo").Handle(rc2); err == nil {
		t.Fatal("expected egress to fail without a core transport")
	}
}

func TestPluginL7ThroughCore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "hello-world")
	}))
	t.Cleanup(srv.Close)

	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")

	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect,
		Net: plugintest.HTTPTransport(srv.URL, http.DefaultTransport),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	rc := plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, nil)
	res, err := routeByID(p, "demo.fetch").Handle(rc)
	if err != nil {
		t.Fatalf("fetch via core L7: %v", err)
	}
	if res.(map[string]any)["body"] != "hello-world" {
		t.Fatalf("unexpected L7 body: %v", res)
	}
}

func TestPluginAuditForwardsToCore(t *testing.T) {
	var mu sync.Mutex
	var got []string
	hook := func(result plugin.AuditResult, params map[string]string, errMsg string) {
		mu.Lock()
		got = append(got, string(result)+"|"+params["op"]+"|"+errMsg)
		mu.Unlock()
	}

	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t), extplugin.WithAudit(hook))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")

	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect,
		Net: plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	rc := plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, nil)
	if _, err := routeByID(p, "demo.audit").Handle(rc); err != nil {
		t.Fatalf("audit route: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != "error|test|boom" {
		t.Fatalf("audit not forwarded to core: %v", got)
	}
}

type testClientStream struct {
	net.Conn
	ctx context.Context
}

func (s *testClientStream) Context() context.Context { return s.ctx }

func TestPluginBidiStream(t *testing.T) {
	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect, Net: plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	testEnd, streamEnd := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &testClientStream{Conn: streamEnd, ctx: ctx}
	rc := plugin.NewRequestContext(ctx, plugin.User{}, sess, nil, nil, nil)
	errCh := make(chan error, 1)
	go func() { errCh <- routeByID(p, "demo.stream").Stream(rc, client) }()

	go func() { _, _ = testEnd.Write([]byte("hello")) }()
	buf := make([]byte, 5)
	if _, err := io.ReadFull(testEnd, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != "hello" {
		t.Fatalf("bidi stream echo got %q", buf)
	}

	cancel()
	_ = testEnd.Close()
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("stream handler did not return after disconnect")
	}
}

func TestPluginOpenChannel(t *testing.T) {
	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect, Net: plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	ch, err := sess.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamLogs})
	if err != nil {
		t.Fatalf("open channel: %v", err)
	}
	defer func() { _ = ch.Close() }()
	if ch.Kind() != plugin.StreamLogs {
		t.Fatalf("channel kind: %v", ch.Kind())
	}
	go func() { _, _ = ch.Write([]byte("ping")) }()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(ch, buf); err != nil {
		t.Fatalf("read channel echo: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("channel echo got %q", buf)
	}
}

func TestPluginHTTPProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream:"+r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect,
		Config: map[string]any{"upstream": upstream.URL},
		Net:    plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	proxier, ok := sess.(plugin.HTTPProxy)
	if !ok {
		t.Fatal("grpcSession should implement plugin.HTTPProxy")
	}
	front := httptest.NewServer(http.HandlerFunc(proxier.ServeHTTPProxy))
	t.Cleanup(front.Close)

	resp, err := front.Client().Get(front.URL + "/page")
	if err != nil {
		t.Fatalf("proxy get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "upstream:/page" {
		t.Fatalf("proxied body got %q (want upstream:/page)", body)
	}
}

// TestExampleMemoLoads proves the docs' promise: the out-of-tree reference plugin
// (its own module, SDK-only) builds and loads with no core changes.
func TestExampleMemoLoads(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "memo")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "../../examples/memo"
	build.Env = append(os.Environ(), "GOWORK=off") // build it as the standalone module it is
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build example: %v\n%s", err, out)
	}

	reg := pluginregistry.New()
	m := extplugin.NewManager(dir)
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, ok := reg.Get("memo")
	if !ok {
		t.Fatal("memo plugin not registered")
	}

	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	create := plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, []byte(`{"title":"hi","body":"there"}`))
	if _, err := routeByID(p, "memo.create").Handle(create); err != nil {
		t.Fatalf("create: %v", err)
	}
	res, err := routeByID(p, "memo.list").Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	items, _ := res.(map[string]any)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["title"] != "hi" {
		t.Fatalf("memo round-trip over subprocess: %v", res)
	}
}

// TestPluginChannelCapabilities asserts Resize and ServerInit survive the wire:
// the host-side channel wrapper re-exposes exactly what the plugin's channel
// declared, matching in-process parity.
func TestPluginChannelCapabilities(t *testing.T) {
	reg := pluginregistry.New()
	m := extplugin.NewManager(buildDemo(t))
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("load: %v", err)
	}
	p, _ := reg.Get("demo")
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{
		ConnectionID: "c1", Transport: plugin.TransportDirect, Net: plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	term, err := sess.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamTerminal})
	if err != nil {
		t.Fatalf("open terminal channel: %v", err)
	}
	defer func() { _ = term.Close() }()
	resizer, ok := term.(interface{ Resize(int, int) error })
	if !ok {
		t.Fatal("terminal channel lost Resize across the wire")
	}
	if err := resizer.Resize(80, 24); err != nil {
		t.Fatalf("resize: %v", err)
	}
	want := "resize:80x24"
	buf := make([]byte, len(want))
	if _, err := io.ReadFull(term, buf); err != nil {
		t.Fatalf("read resize marker: %v", err)
	}
	if string(buf) != want {
		t.Fatalf("resize marker = %q, want %q", buf, want)
	}

	logs, err := sess.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamLogs})
	if err != nil {
		t.Fatalf("open logs channel: %v", err)
	}
	defer func() { _ = logs.Close() }()
	if _, ok := logs.(interface{ Resize(int, int) error }); ok {
		t.Fatal("non-resizable channel must not advertise Resize")
	}

	desktop, err := sess.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamDesktop})
	if err != nil {
		t.Fatalf("open desktop channel: %v", err)
	}
	defer func() { _ = desktop.Close() }()
	si, ok := desktop.(interface{ ServerInit() []byte })
	if !ok {
		t.Fatal("desktop channel lost ServerInit across the wire")
	}
	if got := string(si.ServerInit()); got != "demo-server-init" {
		t.Fatalf("server init = %q", got)
	}
}

func TestLoadOneAndUpdate(t *testing.T) {
	dir := buildDemo(t)
	bin := filepath.Join(dir, "demoplugin")

	reg := pluginregistry.New()
	m := extplugin.NewManager(t.TempDir()) // empty dir: nothing loads at startup
	t.Cleanup(m.Close)
	if err := m.LoadAll(context.Background(), reg); err != nil {
		t.Fatalf("loadall: %v", err)
	}
	if _, ok := reg.Get("demo"); ok {
		t.Fatal("registry must start empty")
	}

	if err := m.LoadOne(context.Background(), reg, bin); err != nil {
		t.Fatalf("LoadOne: %v", err)
	}
	if _, ok := reg.Get("demo"); !ok {
		t.Fatal("demo must be registered after LoadOne")
	}
	if !m.IsManaged("demo") {
		t.Fatal("demo must be managed after LoadOne")
	}

	if err := m.Update(context.Background(), reg, "demo", bin); err != nil {
		t.Fatalf("Update: %v", err)
	}
	p, _ := reg.Get("demo")
	sess, err := p.Connect(context.Background(), plugin.ConnectConfig{ConnectionID: "c1", Transport: plugin.TransportDirect, Net: plugintest.DirectTransport()})
	if err != nil {
		t.Fatalf("connect after update: %v", err)
	}
	defer func() { _ = sess.Close() }()
	route, _ := reg.Route("demo", "demo.list")
	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u"}, sess, map[string]string{"k": "v"}, nil, nil)
	if _, err := route.Handle(rc); err != nil {
		t.Fatalf("invoke after update: %v", err)
	}

	if err := m.Update(context.Background(), reg, "nope", bin); !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("update of unmanaged plugin: want ErrNotFound, got %v", err)
	}
}
