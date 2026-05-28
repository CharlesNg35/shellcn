package vnc

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type fakeNetTransport struct {
	dial func(context.Context, string, string) (net.Conn, error)
}

func (f fakeNetTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f.dial(ctx, network, addr)
}

func (fakeNetTransport) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

func TestHealthCheckAuthenticatesAndClosesProbe(t *testing.T) {
	srv, cli := net.Pipe()
	closed := make(chan struct{})
	go serveVNCNoAuth(t, srv, closed)

	s := &Session{
		net: fakeNetTransport{dial: func(context.Context, string, string) (net.Conn, error) {
			return cli, nil
		}},
		addr: "127.0.0.1:5900",
	}
	if err := s.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("health check should close the probe connection")
	}
}

func serveVNCNoAuth(t *testing.T, conn net.Conn, closed chan<- struct{}) {
	t.Helper()
	defer close(closed)
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("RFB 003.008\n")); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, make([]byte, 12)); err != nil {
		return
	}
	if _, err := conn.Write([]byte{1, 1}); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
		return
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
		return
	}
	init := make([]byte, 24)
	binary.BigEndian.PutUint16(init[0:], 800)
	binary.BigEndian.PutUint16(init[2:], 600)
	_, _ = conn.Write(init)
	_, _ = io.Copy(io.Discard, conn)
}

var _ plugin.NetTransport = fakeNetTransport{}
