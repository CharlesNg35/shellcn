package grpcplugin

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
)

// NewHTTPProxyBridge serves the core's L7 transport over a brokered conn: each
// brokered conn carries one HTTP connection, reverse-proxied to baseURL via rt
// (which injects auth, e.g. agent http_proxy).
func NewHTTPProxyBridge(baseURL string, rt http.RoundTripper) (pluginv1.ConnServer, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = rt
	return NewPipeServer(func(_ context.Context, conn net.Conn) error {
		return http.Serve(newSingleConnListener(conn), proxy)
	}), nil
}

// singleConnListener serves http.Serve a single conn, then blocks until that
// conn closes so Serve exits cleanly.
type singleConnListener struct {
	ch     chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newSingleConnListener(c net.Conn) *singleConnListener {
	l := &singleConnListener{ch: make(chan net.Conn, 1), closed: make(chan struct{})}
	l.ch <- &closeOnceConn{Conn: c, onClose: l.Close}
	return l
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return pipeAddr{} }

type closeOnceConn struct {
	net.Conn
	once    sync.Once
	onClose func() error
}

func (c *closeOnceConn) Close() error {
	c.once.Do(func() { _ = c.onClose() })
	return c.Conn.Close()
}
