package extplugin_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

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
