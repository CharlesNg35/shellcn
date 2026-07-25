package loopback

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

// watchedConn wraps a yamux stream (which has no CloseWrite and whose Close is a
// half-close that does not wake a blocked Read) and signals when a blocked Read
// finally returns, so a test can observe the reverse copy being reaped.
type watchedConn struct {
	net.Conn
	once       sync.Once
	readExited chan struct{}
}

func (w *watchedConn) Read(p []byte) (int, error) {
	n, err := w.Conn.Read(p)
	if err != nil {
		w.once.Do(func() { close(w.readExited) })
	}
	return n, err
}

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

// TestCloseReapsReverseCopyParkedOnYamuxRead proves Bridge.Close releases a
// reverse copy blocked on a yamux up.Read. A yamux Close is only a half-close and
// does not wake that Read, so without the read-deadline reap the copy would
// outlive the bridge until yamux's multi-minute stream-close timeout.
func TestCloseReapsReverseCopyParkedOnYamuxRead(t *testing.T) {
	client, server := yamuxPair(t)
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	readExited := make(chan struct{})
	bridge, err := New(func(context.Context) (net.Conn, error) {
		st, err := client.Open()
		if err != nil {
			return nil, err
		}
		return &watchedConn{Conn: st, readExited: readExited}, nil
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}

	// The agent accepts the stream and stays idle so the gateway reverse copy has
	// nothing to read and parks on up.Read.
	accepted := make(chan net.Conn, 1)
	go func() {
		st, err := server.Accept()
		if err != nil {
			return
		}
		accepted <- st
	}()

	tcpClient, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = tcpClient.Close() }()

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not accept the tunnel stream")
	}

	// Finish the forward direction (client half-close) so only the reverse copy,
	// parked on the idle yamux up.Read, remains.
	if err := tcpClient.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatalf("client CloseWrite: %v", err)
	}

	select {
	case <-readExited:
		t.Fatal("reverse copy was released before bridge close")
	case <-time.After(150 * time.Millisecond):
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("bridge close: %v", err)
	}

	select {
	case <-readExited:
	case <-time.After(3 * time.Second):
		t.Fatal("bridge close did not reap the reverse copy parked on yamux read")
	}
}

func tcpPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		accepted <- c
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if server = <-accepted; server == nil {
		t.Fatal("accept failed")
	}
	return client, server
}

func readExpect(t *testing.T, c net.Conn, want string) {
	t.Helper()
	buf := make([]byte, len(want))
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatalf("read %q: %v", want, err)
	}
	if string(buf) != want {
		t.Fatalf("read = %q, want %q", buf, want)
	}
	_ = c.SetReadDeadline(time.Time{})
}

// TestReverseSurvivesClientHalfClose proves the bridge half-closes rather than
// tearing both directions down on the first EOF: after the client finishes
// sending and half-closes its write, the upstream still delivers its response.
func TestReverseSurvivesClientHalfClose(t *testing.T) {
	upstreams := make(chan net.Conn, 1)
	bridge, err := New(func(context.Context) (net.Conn, error) {
		up, server := tcpPair(t)
		upstreams <- server
		return up, nil
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}
	defer func() { _ = bridge.Close() }()

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = client.Close() }()

	var server net.Conn
	select {
	case server = <-upstreams:
		defer func() { _ = server.Close() }()
	case <-time.After(time.Second):
		t.Fatal("bridge did not dial upstream")
	}

	if _, err := server.Write([]byte("hello")); err != nil {
		t.Fatalf("server write: %v", err)
	}
	readExpect(t, client, "hello")

	if err := client.(*net.TCPConn).CloseWrite(); err != nil {
		t.Fatalf("client CloseWrite: %v", err)
	}

	// The forward half-close propagates as EOF, not a full reset.
	_ = server.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := server.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("upstream read after client half-close = %v, want EOF", err)
	}
	_ = server.SetReadDeadline(time.Time{})

	// The fix: the reverse direction still carries the upstream's response.
	if _, err := server.Write([]byte("world")); err != nil {
		t.Fatalf("server write after half-close: %v", err)
	}
	readExpect(t, client, "world")
}

func TestCloseClosesActiveConnections(t *testing.T) {
	upstreams := make(chan net.Conn, 1)
	bridge, err := New(func(context.Context) (net.Conn, error) {
		local, upstream := net.Pipe()
		upstreams <- upstream
		return local, nil
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = client.Close() }()

	var upstream net.Conn
	select {
	case upstream = <-upstreams:
		defer func() { _ = upstream.Close() }()
	case <-time.After(time.Second):
		t.Fatal("bridge did not dial upstream")
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("close bridge: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("client read should fail after bridge close")
	}

	_ = upstream.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := upstream.Read(make([]byte, 1)); err == nil {
		t.Fatal("upstream read should fail after bridge close")
	}
}

func TestCloseCancelsPendingDial(t *testing.T) {
	dialStarted := make(chan struct{})
	dialDone := make(chan error, 1)
	bridge, err := New(func(ctx context.Context) (net.Conn, error) {
		close(dialStarted)
		<-ctx.Done()
		dialDone <- ctx.Err()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}

	client, err := net.Dial("tcp", bridge.Addr())
	if err != nil {
		t.Fatalf("dial bridge: %v", err)
	}
	defer func() { _ = client.Close() }()

	select {
	case <-dialStarted:
	case <-time.After(time.Second):
		t.Fatal("bridge did not start dial")
	}

	if err := bridge.Close(); err != nil {
		t.Fatalf("close bridge: %v", err)
	}

	select {
	case err := <-dialDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dial err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending dial was not cancelled")
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	_, _ = client.Read(make([]byte, 1))
}
