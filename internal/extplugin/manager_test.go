package extplugin_test

import (
	"context"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/extplugin"
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

	reg := plugin.NewRegistry()
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
	reg := plugin.NewRegistry()
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
	reg := plugin.NewRegistry()
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
