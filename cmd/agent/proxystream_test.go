package main

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"

	"github.com/charlesng35/shellcn/internal/transport"
)

// yamuxPair returns a connected gateway (client) / agent (server) yamux session
// pair over an in-memory pipe, mirroring the real tunnel roles.
func yamuxPair(t *testing.T) (client, server *yamux.Session) {
	t.Helper()
	c, s := net.Pipe()
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = false
	cfg.LogOutput = io.Discard
	var err error
	if client, err = yamux.Client(c, cfg); err != nil {
		t.Fatalf("yamux client: %v", err)
	}
	if server, err = yamux.Server(s, cfg); err != nil {
		t.Fatalf("yamux server: %v", err)
	}
	return client, server
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestProxyStreamReverseSurvivesForwardHalfClose proves the agent no longer
// tears the whole stream down on the first EOF: after the gateway half-closes
// its write (an upload's stdin EOF), the target's reply still flows back over
// the same yamux stream.
func TestProxyStreamReverseSurvivesForwardHalfClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Read the whole forward stream (ends at the client half-close), then reply.
		got, _ := io.ReadAll(conn)
		_, _ = conn.Write(append([]byte("reply:"), got...))
	}()

	client, server := yamuxPair(t)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		st, err := server.Accept()
		if err != nil {
			return
		}
		proxyStream(discardLogger(), st, transport.AgentProxyTarget{Mode: "tcp", Address: ln.Addr().String()})
	}()

	st, err := client.Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := st.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Half-close the forward direction (FIN); the reverse must keep flowing.
	if err := st.Close(); err != nil {
		t.Fatalf("half-close: %v", err)
	}

	_ = st.SetReadDeadline(time.Now().Add(3 * time.Second))
	reply, err := io.ReadAll(st)
	if err != nil {
		t.Fatalf("read reply after half-close: %v", err)
	}
	if string(reply) != "reply:hello" {
		t.Fatalf("reply = %q, want %q", reply, "reply:hello")
	}
}

// TestProxyStreamClosesTargetOnSessionTeardown proves the half-close-aware copy
// does not leak: with the reverse copy parked on an idle target's Read, a yamux
// session teardown still unblocks it, returns proxyStream, and releases the
// target connection. Without the fix, proxyStream would hang on up.Read forever.
func TestProxyStreamClosesTargetOnSessionTeardown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	targets := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		targets <- conn // stay idle: never write, never close
	}()

	client, server := yamuxPair(t)
	defer func() { _ = client.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		st, err := server.Accept()
		if err != nil {
			return
		}
		proxyStream(discardLogger(), st, transport.AgentProxyTarget{Mode: "tcp", Address: ln.Addr().String()})
	}()

	st, err := client.Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Half-close the forward direction (no payload, so nothing is buffered at the
	// target) so only the reverse copy remains, parked on the idle target's Read.
	if err := st.Close(); err != nil {
		t.Fatalf("half-close: %v", err)
	}

	var target net.Conn
	select {
	case target = <-targets:
		defer func() { _ = target.Close() }()
	case <-time.After(2 * time.Second):
		t.Fatal("proxyStream did not dial the target")
	}

	// Tear the session down (a dropped tunnel). proxyStream must return.
	_ = server.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("proxyStream leaked: did not return after session teardown")
	}

	// The fix closed the target conn, so the peer now observes EOF/reset.
	_ = target.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := target.Read(make([]byte, 1)); err == nil {
		t.Fatal("target connection still open after session teardown")
	}
}
