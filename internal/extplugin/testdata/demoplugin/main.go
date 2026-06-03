// Command demoplugin is a minimal out-of-tree plugin used by the extplugin
// end-to-end tests.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/charlesng35/shellcn/sdk"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type demo struct{}

func (demo) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "demo",
		Title:               "Demo",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{{
			Key: "data", Label: "Data", Type: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "demo.list"},
			Config: plugin.TableConfig{Editable: true, RowKey: []string{"id"}},
		}},
		Streams: []plugin.Stream{{ID: "demo.stream", Kind: plugin.StreamLogs, RouteID: "demo.stream"}},
	}
}

func (demo) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "demo.list", Method: plugin.MethodGet, Path: "/list",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.list",
			Handle: func(rc *plugin.RequestContext) (any, error) {
				return map[string]any{"items": []string{"alpha", rc.Param("k")}}, nil
			},
		},
		{
			ID: "demo.echo", Method: plugin.MethodPost, Path: "/echo",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.echo",
			Handle: echo,
		},
		{
			ID: "demo.fetch", Method: plugin.MethodGet, Path: "/fetch",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.fetch",
			Handle: fetch,
		},
		{
			ID: "demo.audit", Method: plugin.MethodPost, Path: "/audit",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.audit",
			Handle: func(rc *plugin.RequestContext) (any, error) {
				rc.Audit(plugin.AuditError, map[string]string{"op": "test"}, fmt.Errorf("boom"))
				return map[string]any{"ok": true}, nil
			},
		},
		{
			ID: "demo.stream", Method: plugin.MethodWS, Path: "/stream",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.stream",
			Stream: func(_ *plugin.RequestContext, c plugin.ClientStream) error {
				_, err := io.Copy(c, c)
				return err
			},
		},
		{
			ID: "demo.crash", Method: plugin.MethodPost, Path: "/crash",
			Permission: "demo.read", Risk: plugin.RiskSafe, AuditEvent: "demo.crash",
			Handle: func(*plugin.RequestContext) (any, error) { os.Exit(1); return nil, nil },
		},
	}
}

// fetch reaches an HTTP target through the core's L7 transport.
func fetch(rc *plugin.RequestContext) (any, error) {
	s := rc.Session.(*demoSession)
	base, rt, ok := s.transport.HTTP()
	if !ok {
		return nil, fmt.Errorf("%w: no L7 transport", plugin.ErrUnavailable)
	}
	resp, err := (&http.Client{Transport: rt}).Get(base + "/hello")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return map[string]any{"body": string(body)}, nil
}

// echo dials the configured target through the core-provided transport and
// returns its echo, exercising brokered egress.
func echo(rc *plugin.RequestContext) (any, error) {
	s := rc.Session.(*demoSession)
	if s.transport == nil {
		return nil, fmt.Errorf("%w: no transport", plugin.ErrUnavailable)
	}
	conn, err := s.transport.DialContext(rc.Ctx, "tcp", s.target)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write(rc.Body()); err != nil {
		return nil, err
	}
	buf := make([]byte, len(rc.Body()))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return map[string]any{"echo": string(buf)}, nil
}

func (demo) Connect(_ context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	return &demoSession{transport: cfg.Net, target: cfg.String("target"), upstream: cfg.String("upstream")}, nil
}

type demoSession struct {
	transport plugin.NetTransport
	target    string
	upstream  string
}

// ServeHTTPProxy reverse-proxies to the configured upstream through the core's
// transport, exercising the "open in browser" bridge.
func (s *demoSession) ServeHTTPProxy(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(s.upstream)
	if err != nil || s.upstream == "" {
		http.Error(w, "no upstream", http.StatusBadGateway)
		return
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = &http.Transport{DialContext: s.transport.DialContext}
	rp.ServeHTTP(w, r)
}

func (demoSession) HealthCheck(context.Context) error { return nil }

// OpenChannel returns an echo channel so the channel bridge can be exercised.
// Terminal channels are resizable and desktop channels carry a server-init blob,
// so the capability passthrough can be asserted end to end.
func (demoSession) OpenChannel(_ context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	a, b := net.Pipe()
	go func() { _, _ = io.Copy(b, b); _ = b.Close() }()
	base := &pipeChannel{conn: a, kind: req.Kind}
	switch req.Kind {
	case plugin.StreamTerminal:
		return &resizablePipeChannel{pipeChannel: base}, nil
	case plugin.StreamDesktop:
		return &desktopPipeChannel{pipeChannel: base, init: []byte("demo-server-init")}, nil
	default:
		return base, nil
	}
}

// resizablePipeChannel echoes resize calls into the stream so a test can observe
// that Resize crossed the wire.
type resizablePipeChannel struct{ *pipeChannel }

func (c *resizablePipeChannel) Resize(cols, rows int) error {
	_, err := c.conn.Write(fmt.Appendf(nil, "resize:%dx%d", cols, rows))
	return err
}

type desktopPipeChannel struct {
	*pipeChannel
	init []byte
}

func (c *desktopPipeChannel) ServerInit() []byte { return c.init }

func (demoSession) Close() error { return nil }

type pipeChannel struct {
	conn net.Conn
	kind plugin.StreamKind
}

func (c *pipeChannel) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *pipeChannel) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *pipeChannel) Close() error                { return c.conn.Close() }
func (c *pipeChannel) Kind() plugin.StreamKind     { return c.kind }

func main() { sdk.Serve(demo{}) }
