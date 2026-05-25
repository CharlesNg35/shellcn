package transport_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/charlesng/shellcn/internal/transport"
)

// TestTunnelEndToEnd wires a gateway yamux client to an agent yamux server over
// an in-memory pipe, registers the dialer, then dials through it to an echo
// target — proving the gateway reaches a target only the agent can dial.
func TestTunnelEndToEnd(t *testing.T) {
	// Echo "target" the agent proxies to.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(conn, conn); _ = conn.Close() }()
		}
	}()

	gatewaySide, agentSide := net.Pipe()

	// Agent side: yamux server accepts streams and proxies them to the echo target.
	go func() {
		sess, err := yamux.Server(agentSide, agentYamuxConfig())
		if err != nil {
			return
		}
		for {
			stream, err := sess.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = stream.Close() }()
				up, err := net.Dial("tcp", ln.Addr().String())
				if err != nil {
					return
				}
				defer func() { _ = up.Close() }()
				done := make(chan struct{}, 2)
				go func() { _, _ = io.Copy(up, stream); done <- struct{}{} }()
				go func() { _, _ = io.Copy(stream, up); done <- struct{}{} }()
				<-done
			}()
		}
	}()

	// Gateway side: yamux client, registered as a dialer.
	reg := transport.NewRegistry()
	sess, err := yamux.Client(gatewaySide, agentYamuxConfig())
	if err != nil {
		t.Fatalf("yamux client: %v", err)
	}
	defer func() { _ = sess.Close() }()
	reg.Register("c1", func(context.Context, string, string) (net.Conn, error) { return sess.Open() })

	dial, ok := reg.Dialer("c1")
	if !ok {
		t.Fatal("dialer not registered")
	}
	conn, err := dial(context.Background(), "tcp", "ignored")
	if err != nil {
		t.Fatalf("dial through tunnel: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("through-the-tunnel")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf[:n]) != "through-the-tunnel" {
		t.Errorf("tunnel echo mismatch: got %q", buf[:n])
	}
}

func agentYamuxConfig() *yamux.Config {
	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard
	return cfg
}
