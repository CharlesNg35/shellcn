package transport_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/transport"
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
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "direct"}, transport.EmptyTunnelRegistry{})
	if err != nil {
		t.Fatalf("build direct: %v", err)
	}
	if nt == nil {
		t.Fatal("nil transport")
	}
	// Empty transport string defaults to direct.
	if _, err := transport.Build(models.Connection{ID: "c2"}, transport.EmptyTunnelRegistry{}); err != nil {
		t.Errorf("empty transport should default to direct: %v", err)
	}
}

func TestBuildAgentUnavailableInM1(t *testing.T) {
	_, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, transport.EmptyTunnelRegistry{})
	if !errors.Is(err, transport.ErrAgentUnavailable) {
		t.Errorf("agent with no tunnel: want ErrAgentUnavailable, got %v", err)
	}
}

func TestBuildAgentWithTunnel(t *testing.T) {
	reg := stubRegistry{dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dialed")
	}}
	nt, err := transport.Build(models.Connection{ID: "c1", Transport: "agent"}, reg)
	if err != nil {
		t.Fatalf("build agent: %v", err)
	}
	if _, err := nt.DialContext(context.Background(), "tcp", "x"); err == nil || err.Error() != "dialed" {
		t.Errorf("agent transport should route through the tunnel dialer, got %v", err)
	}
}

func TestBuildUnknownMode(t *testing.T) {
	if _, err := transport.Build(models.Connection{ID: "c1", Transport: "carrier-pigeon"}, nil); err == nil {
		t.Error("unknown transport mode should error")
	}
}

type stubRegistry struct{ dial transport.DialFunc }

func (s stubRegistry) Dialer(string) (transport.DialFunc, bool) { return s.dial, true }
