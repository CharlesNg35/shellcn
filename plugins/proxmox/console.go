package proxmox

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	pmox "github.com/luthermonson/go-proxmox"

	"github.com/charlesng35/shellcn/plugins/shared/rfb"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// openVMConsole opens a QEMU VNC console: it asks the API for a one-shot
// vnc ticket/port, dials the matching vncwebsocket, and authenticates RFB with
// the ticket — so the password never leaves the gateway. The raw RFB stream is
// then spliced to the browser's noVNC client.
func (s *Session) openVMConsole(ctx context.Context, node, vmid string) (plugin.Channel, error) {
	if node == "" || vmid == "" {
		return nil, fmt.Errorf("%w: node and vmid are required", plugin.ErrInvalidInput)
	}
	var vnc pmox.VNC
	if err := s.client.Post(ctx, fmt.Sprintf("/nodes/%s/qemu/%s/vncproxy", node, vmid), pmox.VNCConfig{Websocket: true}, &vnc); err != nil {
		return nil, fmt.Errorf("%w: open vnc proxy: %v", plugin.ErrUnavailable, err)
	}
	wsPath := fmt.Sprintf("/nodes/%s/qemu/%s/vncwebsocket?port=%d&vncticket=%s", node, vmid, int(vnc.Port), url.QueryEscape(vnc.Ticket))
	c, err := s.dialWS(ctx, wsPath)
	if err != nil {
		return nil, err
	}
	connCtx, cancel := context.WithCancel(context.Background())
	conn := websocket.NetConn(connCtx, c, websocket.MessageBinary)
	serverInit, err := rfb.DialVNC(conn, vnc.Ticket)
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, fmt.Errorf("%w: vnc handshake: %v", plugin.ErrUnavailable, err)
	}
	return &desktopChannel{conn: conn, serverInit: serverInit, cancel: cancel}, nil
}

// openTerminal opens an LXC or node shell over termproxy. The vncwebsocket then
// carries Proxmox's terminal framing, adapted here to a plain byte stream for the
// xterm panel.
func (s *Session) openTerminal(ctx context.Context, params map[string]string) (plugin.Channel, error) {
	node := params["node"]
	if node == "" {
		return nil, fmt.Errorf("%w: node is required", plugin.ErrInvalidInput)
	}
	var proxyPath, wsPath string
	switch params["kind"] {
	case "lxc":
		vmid := params["vmid"]
		if vmid == "" {
			return nil, fmt.Errorf("%w: vmid is required", plugin.ErrInvalidInput)
		}
		proxyPath = fmt.Sprintf("/nodes/%s/lxc/%s/termproxy", node, vmid)
		wsPath = fmt.Sprintf("/nodes/%s/lxc/%s/vncwebsocket", node, vmid)
	case "node":
		proxyPath = fmt.Sprintf("/nodes/%s/termproxy", node)
		wsPath = fmt.Sprintf("/nodes/%s/vncwebsocket", node)
	default:
		return nil, fmt.Errorf("%w: unsupported terminal kind", plugin.ErrInvalidInput)
	}

	var term pmox.Term
	if err := s.client.Post(ctx, proxyPath, nil, &term); err != nil {
		return nil, fmt.Errorf("%w: open term proxy: %v", plugin.ErrUnavailable, err)
	}
	wsPath = fmt.Sprintf("%s?port=%d&vncticket=%s", wsPath, int(term.Port), url.QueryEscape(term.Ticket))
	c, err := s.dialWS(ctx, wsPath)
	if err != nil {
		return nil, err
	}
	connCtx, cancel := context.WithCancel(context.Background())
	tc := &termChannel{c: c, ctx: connCtx, cancel: cancel}
	if err := tc.authenticate(term.User, term.Ticket); err != nil {
		_ = tc.Close()
		return nil, fmt.Errorf("%w: termproxy auth: %v", plugin.ErrUnavailable, err)
	}
	go tc.keepAlive()
	return tc, nil
}

func (s *Session) dialWS(ctx context.Context, path string) (*websocket.Conn, error) {
	header := http.Header{}
	s.apply(header)
	c, resp, err := websocket.Dial(ctx, s.wsBase+path, &websocket.DialOptions{
		HTTPClient:   s.httpc,
		HTTPHeader:   header,
		Subprotocols: []string{"binary"},
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: dial proxmox console: %v", plugin.ErrUnavailable, err)
	}
	c.SetReadLimit(-1) // RFB framebuffer updates routinely exceed the 32 KiB default.
	return c, nil
}

// desktopChannel carries the authenticated upstream RFB byte stream to noVNC.
type desktopChannel struct {
	conn       net.Conn
	serverInit []byte
	cancel     context.CancelFunc
	once       sync.Once
}

func (c *desktopChannel) Kind() plugin.StreamKind     { return plugin.StreamDesktop }
func (c *desktopChannel) Read(p []byte) (int, error)  { return c.conn.Read(p) }
func (c *desktopChannel) Write(p []byte) (int, error) { return c.conn.Write(p) }
func (c *desktopChannel) ServerInit() []byte          { return c.serverInit }

func (c *desktopChannel) Close() error {
	c.once.Do(c.cancel)
	return c.conn.Close()
}

// termChannel adapts Proxmox's termproxy framing to a plain terminal byte stream.
// Server output arrives as raw bytes; client input is framed as `0:<len>:<data>`
// and resizes as `1:<rows>:<cols>:`, each in its own binary websocket message.
type termChannel struct {
	c      *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	wmu    sync.Mutex
	rbuf   []byte
	once   sync.Once
}

func (t *termChannel) Kind() plugin.StreamKind { return plugin.StreamTerminal }

func (t *termChannel) authenticate(user, ticket string) error {
	if err := t.write([]byte(user + ":" + ticket + "\n")); err != nil {
		return err
	}
	_, msg, err := t.c.Read(t.ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(msg)) != "OK" {
		return fmt.Errorf("authentication rejected")
	}
	return nil
}

func (t *termChannel) Read(p []byte) (int, error) {
	if len(t.rbuf) == 0 {
		_, data, err := t.c.Read(t.ctx)
		if err != nil {
			return 0, err
		}
		t.rbuf = data
	}
	n := copy(p, t.rbuf)
	t.rbuf = t.rbuf[n:]
	return n, nil
}

func (t *termChannel) Write(p []byte) (int, error) {
	if err := t.write(inputFrame(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Resize forwards an xterm resize as the termproxy `1:rows:cols:` control frame.
func (t *termChannel) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return t.write(resizeFrame(cols, rows))
}

// inputFrame wraps terminal input in Proxmox's `0:<len>:<data>` framing.
func inputFrame(p []byte) []byte { return append(fmt.Appendf(nil, "0:%d:", len(p)), p...) }

// resizeFrame builds Proxmox's `1:<rows>:<cols>:` terminal-resize control frame.
func resizeFrame(cols, rows int) []byte { return fmt.Appendf(nil, "1:%d:%d:", rows, cols) }

func (t *termChannel) write(b []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	return t.c.Write(t.ctx, websocket.MessageBinary, b)
}

func (t *termChannel) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			if err := t.write([]byte("2")); err != nil {
				return
			}
		}
	}
}

func (t *termChannel) Close() error {
	t.once.Do(t.cancel)
	return t.c.Close(websocket.StatusNormalClosure, "")
}
