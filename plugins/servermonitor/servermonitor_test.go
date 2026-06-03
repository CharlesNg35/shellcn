package servermonitor

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	if err := plugin.Validate(New().Manifest(), New().Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func TestDirectCollection(t *testing.T) {
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportDirect,
		Config:    map[string]any{"process_limit": 50, "metrics_interval_seconds": 1},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, nil)
	out, err := Overview(rc)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	overview, ok := out.(map[string]any)
	if !ok || overview["hostname"] == "" {
		t.Fatalf("overview = %#v", out)
	}
}

func TestRemoteAgentCollection(t *testing.T) {
	local := hostmonitor.NewLocal(hostmonitor.Options{ProcessLimit: 10})
	srv := httptest.NewServer(hostmonitor.Handler(local))
	defer srv.Close()

	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportAgent,
		Config:    map[string]any{"metrics_interval_seconds": 1},
		Net:       fakeHostMonitorNet{baseURL: srv.URL},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, nil)
	out, err := Processes(rc)
	if err != nil {
		t.Fatalf("processes: %v", err)
	}
	page, ok := out.(plugin.Page[hostmonitor.Row])
	if !ok {
		t.Fatalf("processes returned %T", out)
	}
	if page.Total == nil {
		b, _ := json.Marshal(out)
		t.Fatalf("missing total in %s", b)
	}
}

type fakeHostMonitorNet struct {
	baseURL string
}

func (f fakeHostMonitorNet) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, nil
}

func (f fakeHostMonitorNet) HTTP() (string, http.RoundTripper, bool) {
	return f.baseURL, http.DefaultTransport, true
}
