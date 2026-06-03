// Package plugintest provides test helpers for exercising a plugin's
// Connect/Session against a real target, without depending on the core.
package plugintest

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// HTTPTransport returns a plugin.NetTransport that offers an L7 client (base URL
// + RoundTripper), like the core's agent http_proxy mode, for testing a plugin's
// HTTP egress. DialContext (L4) is unavailable.
func HTTPTransport(baseURL string, rt http.RoundTripper) plugin.NetTransport {
	return httpTransport{baseURL: baseURL, rt: rt}
}

type httpTransport struct {
	baseURL string
	rt      http.RoundTripper
}

func (httpTransport) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("plugintest: L7-only transport has no DialContext")
}

func (t httpTransport) HTTP() (string, http.RoundTripper, bool) { return t.baseURL, t.rt, true }

// DirectTransport returns a plugin.NetTransport that dials targets directly,
// for driving a plugin Connect/Session in tests. It has no egress allow-list,
// so a test may reach any address it sets up.
func DirectTransport() plugin.NetTransport {
	return directTransport{dialer: &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}}
}

type directTransport struct{ dialer *net.Dialer }

func (d directTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, addr)
}

func (directTransport) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }

// TransportFunc returns a plugin.NetTransport whose DialContext delegates to
// dial — for exercising a plugin over a custom or agent-style transport in tests
// without standing up the core's tunnel registry.
func TransportFunc(dial func(ctx context.Context, network, addr string) (net.Conn, error)) plugin.NetTransport {
	return funcTransport{dial: dial}
}

type funcTransport struct {
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (f funcTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f.dial(ctx, network, addr)
}

func (funcTransport) HTTP() (string, http.RoundTripper, bool) { return "", nil, false }
