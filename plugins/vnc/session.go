package vnc

import (
	"context"
	"fmt"
	"io"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/rfb"
)

// Session holds the per-connection VNC dial parameters. VNC keeps no persistent
// upstream socket; each desktop channel dials fresh and authenticates.
type Session struct {
	net      plugin.NetTransport
	addr     string
	password string
}

func (s *Session) HealthCheck(context.Context) error { return nil }

func (s *Session) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	if req.Kind != plugin.StreamDesktop {
		return nil, plugin.ErrNotSupported
	}
	conn, err := s.net.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("%w: dial vnc target: %v", plugin.ErrUnavailable, err)
	}
	serverInit, err := rfb.DialVNC(conn, s.password)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	}
	return &desktopChannel{conn: conn, serverInit: serverInit}, nil
}

func (s *Session) Close() error { return nil }

// desktopChannel carries the authenticated upstream RFB byte stream.
type desktopChannel struct {
	conn       io.ReadWriteCloser
	serverInit []byte
}

func (c *desktopChannel) Kind() plugin.StreamKind     { return plugin.StreamDesktop }
func (c *desktopChannel) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *desktopChannel) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *desktopChannel) Close() error                { return c.conn.Close() }
func (c *desktopChannel) ServerInit() []byte          { return c.serverInit }

// desktopStream bridges the browser's noVNC client to the authenticated upstream
// RFB session: it completes the gateway-side handshake (Security None), forwards
// the upstream ServerInit, then splices the raw RFB byte stream both ways.
func desktopStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamDesktop})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	si, ok := ch.(interface{ ServerInit() []byte })
	if !ok {
		return fmt.Errorf("%w: desktop channel missing server init", plugin.ErrUnavailable)
	}
	if err := rfb.ServerHandshakeNone(client); err != nil {
		return err
	}
	if _, err := client.Write(si.ServerInit()); err != nil {
		return err
	}

	errc := make(chan error, 2)
	go func() { _, err := io.Copy(client, ch); errc <- err }()
	go func() { _, err := io.Copy(ch, client); errc <- err }()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		if err == io.EOF {
			return nil
		}
		return err
	}
}
