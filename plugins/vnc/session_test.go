package vnc

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
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

func TestParseConnectOptionsRejectsURLHostAndUnknownAuth(t *testing.T) {
	base := map[string]any{"host": "https://vnc.example", "password": "p"}
	if _, err := parseConnectOptions(plugin.ConnectConfig{Config: base}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("URL host should fail as invalid input, got %v", err)
	}
	base["host"] = "vnc.example"
	base["auth"] = "token"
	if _, err := parseConnectOptions(plugin.ConnectConfig{Config: base}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("unknown auth should fail as invalid input, got %v", err)
	}
}

func TestParseConnectOptionsRequiresPasswordUnlessAuthNone(t *testing.T) {
	if _, err := parseConnectOptions(plugin.ConnectConfig{Config: map[string]any{"host": "vnc.example"}}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("missing password should fail as invalid input, got %v", err)
	}
	opts, err := parseConnectOptions(plugin.ConnectConfig{Config: map[string]any{"host": "vnc.example", "auth": "none"}})
	if err != nil {
		t.Fatalf("auth none should not require password: %v", err)
	}
	if opts.Password != "" {
		t.Fatalf("auth none password = %q, want empty", opts.Password)
	}
}

func TestManifestEnablesVNCResize(t *testing.T) {
	tab := New().Manifest().Tabs[0]
	cfg, ok := tab.Config.(plugin.RemoteDesktopConfig)
	if !ok {
		t.Fatalf("remote desktop config = %T", tab.Config)
	}
	if !cfg.Resize {
		t.Fatalf("vnc remote desktop config = %#v, want resize enabled", cfg)
	}
	if cfg.Clipboard {
		t.Fatalf("vnc remote desktop config = %#v, clipboard is not implemented in renderer", cfg)
	}
}
