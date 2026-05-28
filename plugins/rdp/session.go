package rdp

import (
	"context"
	"fmt"
	"io"

	gclient "github.com/x90skysn3k/grdp/client"
	"github.com/x90skysn3k/grdp/glog"

	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/plugins/shared/rfb"
)

// Session holds the per-connection RDP dial parameters. grdp opens its own TCP
// connection, so RDP supports direct transport only.
type Session struct {
	addr     string
	user     string
	password string
	width    int
	height   int
}

func (s *Session) HealthCheck(ctx context.Context) error {
	setting := gclient.NewSetting()
	setting.Width = s.width
	setting.Height = s.height
	setting.LogLevel = glog.NONE
	g := gclient.NewClient(s.addr, s.user, s.password, gclient.TC_RDP, setting)
	defer g.Close()
	if err := g.LoginContext(ctx); err != nil {
		return fmt.Errorf("%w: rdp login failed: %v", plugin.ErrUnauthorized, err)
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error { return nil }

// rdpSession recovers the concrete Session from rc.Session, looking through the
// core's borrowed Handle (which exposes the live session via Session()).
func rdpSession(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	if h, ok := sess.(interface{ Session() plugin.Session }); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: rdp session unavailable", plugin.ErrUnavailable)
}

// bridge translates the browser's RFB input into grdp input events. Its methods
// are only called from the framebuffer server's single read loop, so the
// button-mask state needs no locking.
type bridge struct {
	client   *gclient.Client
	prevMask uint8
}

func (b *bridge) KeyEvent(down bool, keysym uint32) {
	sc, ok := keysymToScancode[keysym]
	if !ok {
		return
	}
	if down {
		b.client.KeyDown(sc, "")
	} else {
		b.client.KeyUp(sc, "")
	}
}

func (b *bridge) PointerEvent(mask uint8, x, y int) {
	b.client.MouseMove(x, y)
	transition := func(bit uint8, button int) {
		was := b.prevMask&bit != 0
		now := mask&bit != 0
		switch {
		case now && !was:
			b.client.MouseDown(button, x, y)
		case !now && was:
			b.client.MouseUp(button, x, y)
		}
	}
	transition(0x1, 0) // left
	transition(0x2, 1) // middle
	transition(0x4, 2) // right
	if mask&0x8 != 0 {
		b.client.MouseWheel(-120, x, y) // wheel up
	}
	if mask&0x10 != 0 {
		b.client.MouseWheel(120, x, y) // wheel down
	}
	b.prevMask = mask & 0x7
}

// desktopStream connects to the RDP host via grdp (credentials stay server-side)
// and bridges its decoded framebuffer to the browser's noVNC client as a
// synthetic RFB session.
func desktopStream(rc *plugin.RequestContext, client plugin.ClientStream) error {
	sess, err := rdpSession(rc.Session)
	if err != nil {
		return err
	}

	setting := gclient.NewSetting()
	setting.Width = sess.width
	setting.Height = sess.height
	setting.LogLevel = glog.NONE
	g := gclient.NewClient(sess.addr, sess.user, sess.password, gclient.TC_RDP, setting)

	if err := g.LoginContext(rc.Ctx); err != nil {
		return fmt.Errorf("%w: rdp login failed: %v", plugin.ErrUnauthorized, err)
	}
	defer g.Close()

	fb := rfb.NewFramebufferServer(client, sess.width, sess.height)
	upstream := make(chan error, 1)
	g.OnError(func(e error) { trySend(upstream, e) })
	g.OnClose(func() { trySend(upstream, io.EOF) })
	g.OnBitmap(func(bitmaps []gclient.Bitmap) {
		for _, bm := range bitmaps {
			fb.PushBitmap(bm.DestLeft, bm.DestTop, bm.Width, bm.Height, bm.BitsPerPixel, bm.Data)
		}
	})

	served := make(chan error, 1)
	go func() { served <- fb.Serve(&bridge{client: g}) }()

	select {
	case <-client.Context().Done():
		return nil
	case err := <-served:
		return err
	case err := <-upstream:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func trySend(ch chan error, err error) {
	select {
	case ch <- err:
	default:
	}
}
