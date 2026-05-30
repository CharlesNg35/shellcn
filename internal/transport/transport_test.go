package transport_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/transport"
)

func TestDirectDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()

	d := transport.NewDirect()
	conn, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()

	if _, _, ok := d.HTTP(); ok {
		t.Error("direct HTTP() should report ok=false (L7 is the plugin's concern)")
	}
}

func TestBuildDirect(t *testing.T) {
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "direct"}, transport.NewRegistry(), "")
	if err != nil {
		t.Fatalf("build direct: %v", err)
	}
	if nt == nil {
		t.Fatal("nil transport")
	}
	// Empty transport string defaults to direct.
	if _, err := transport.Build(models.Connection{ID: "c2"}, transport.NewRegistry(), ""); err != nil {
		t.Errorf("empty transport should default to direct: %v", err)
	}
}

func TestDirectForConnectionOnlyDialsDeclaredTarget(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()
	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	d := transport.NewDirectForConnection(models.Connection{Config: map[string]any{"host": host, "port": port}})
	c, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("declared dial: %v", err)
	}
	_ = c.Close()

	if _, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort(host, "1")); err == nil {
		t.Fatal("dial to undeclared port should be rejected")
	}

	empty := transport.NewDirectForConnection(models.Connection{})
	if _, err := empty.DialContext(context.Background(), "tcp", ln.Addr().String()); err == nil {
		t.Fatal("connection with no declared target should not direct-dial arbitrary addresses")
	}
}

func TestDirectForConnectionAllowsLoopbackAliases(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	d := transport.NewDirectForConnection(models.Connection{Config: map[string]any{"urls": "nats://localhost:" + port}})
	c, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("loopback alias dial: %v", err)
	}
	_ = c.Close()
}

func TestDirectForConnectionAllowsDeclaredPortRange(t *testing.T) {
	control, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen control: %v", err)
	}
	defer func() { _ = control.Close() }()
	data, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen data: %v", err)
	}
	defer func() { _ = data.Close() }()
	go func() {
		c, err := data.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()
	host, controlPort, err := net.SplitHostPort(control.Addr().String())
	if err != nil {
		t.Fatalf("split control addr: %v", err)
	}
	_, dataPort, err := net.SplitHostPort(data.Addr().String())
	if err != nil {
		t.Fatalf("split data addr: %v", err)
	}
	d := transport.NewDirectForConnection(models.Connection{Config: map[string]any{
		"host":               host,
		"port":               controlPort,
		"passive_port_start": dataPort,
		"passive_port_end":   dataPort,
	}})
	c, err := d.DialContext(context.Background(), "tcp", data.Addr().String())
	if err != nil {
		t.Fatalf("declared range dial: %v", err)
	}
	_ = c.Close()
}

func TestBuildAgentUnavailableWithoutTunnel(t *testing.T) {
	_, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, transport.NewRegistry(), "")
	if !errors.Is(err, transport.ErrAgentUnavailable) {
		t.Errorf("agent with no tunnel: want ErrAgentUnavailable, got %v", err)
	}
}

func TestRegistryRegisterResolveRemove(t *testing.T) {
	reg := transport.NewRegistry()
	if _, ok := reg.Dialer("c1"); ok {
		t.Error("empty registry should have no dialer")
	}
	reg.Register("c1", func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("via tunnel") })

	// An agent-mode connection now resolves through the registered dialer.
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg, "")
	if err != nil {
		t.Fatalf("build agent with registered tunnel: %v", err)
	}
	_, derr := nt.DialContext(context.Background(), "tcp", "x")
	if derr == nil || !errors.Is(derr, transport.ErrAgentUnavailable) || !strings.Contains(derr.Error(), "via tunnel") {
		t.Errorf("expected dial through tunnel flagged unavailable, got %v", derr)
	}

	reg.Remove("c1")
	if _, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg, ""); !errors.Is(err, transport.ErrAgentUnavailable) {
		t.Errorf("after Remove: want ErrAgentUnavailable, got %v", err)
	}
}

func TestBuildAgentWithTunnel(t *testing.T) {
	reg := stubRegistry{dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dialed")
	}}
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg, "")
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}
	// The dialer is reached ("dialed"), and a failure through it is flagged so the
	// boundary can report the agent as offline rather than a generic 500.
	_, err = nt.DialContext(context.Background(), "tcp", "x")
	if err == nil || !errors.Is(err, transport.ErrAgentUnavailable) || !strings.Contains(err.Error(), "dialed") {
		t.Errorf("agent transport should route through the tunnel dialer, got %v", err)
	}
}

func TestBuildAgentL4ModesHaveNoHTTP(t *testing.T) {
	reg := stubRegistry{dial: func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("dialed") }}
	for _, mode := range []plugin.AgentMode{plugin.AgentTCP, plugin.AgentUnix, ""} {
		nt, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg, mode)
		if err != nil {
			t.Fatalf("build agent %q: %v", mode, err)
		}
		if _, _, ok := nt.HTTP(); ok {
			t.Errorf("agent mode %q should report HTTP() ok=false", mode)
		}
	}
}

func TestBuildAgentHTTPProxyExposesL7(t *testing.T) {
	dialed := make(chan struct{}, 1)
	reg := stubRegistry{dial: func(context.Context, string, string) (net.Conn, error) {
		select {
		case dialed <- struct{}{}:
		default:
		}
		return nil, errors.New("tunnel dialed")
	}}
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg, plugin.AgentHTTP)
	if err != nil {
		t.Fatalf("build agent http_proxy: %v", err)
	}
	baseURL, rt, ok := nt.HTTP()
	if !ok || rt == nil {
		t.Fatalf("http_proxy mode should expose an L7 transport, got ok=%v rt=%v", ok, rt)
	}
	if baseURL == "" {
		t.Fatal("http_proxy base URL must be non-empty")
	}
	// The RoundTripper must dial the tunnel, not the network directly.
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/namespaces", nil)
	if resp, _ := rt.RoundTrip(req); resp != nil {
		_ = resp.Body.Close()
	}
	select {
	case <-dialed:
	default:
		t.Error("L7 RoundTripper did not route through the tunnel dialer")
	}
}

func TestBuildUnknownMode(t *testing.T) {
	if _, err := transport.Build(models.Connection{ID: "c1", Transport: "carrier-pigeon"}, nil, ""); err == nil {
		t.Error("unknown transport mode should error")
	}
}

type stubRegistry struct{ dial transport.DialFunc }

func (s stubRegistry) Dialer(string) (transport.DialFunc, bool) { return s.dial, true }
