package grpcplugin

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
)

// httpProxyBridge serves one HTTP connection over a brokered Conn.Pipe stream,
// reverse-proxying requests to the connection's L7 base URL via the core's
// RoundTripper (which injects auth, e.g. agent http_proxy).
type httpProxyBridge struct {
	pluginv1.UnimplementedConnServer
	proxy *httputil.ReverseProxy
}

// NewHTTPProxyBridge serves the core's L7 transport over a brokered conn.
func NewHTTPProxyBridge(baseURL string, rt http.RoundTripper) (pluginv1.ConnServer, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = rt
	return &httpProxyBridge{proxy: proxy}, nil
}

func (b *httpProxyBridge) Pipe(stream pluginv1.Conn_PipeServer) error {
	_ = http.Serve(newSingleConnListener(newStreamConn(stream, nil)), b.proxy)
	return nil
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
