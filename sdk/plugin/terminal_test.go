package plugin_test

import (
	"context"
	"io"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type recordingChannel struct {
	wrote  []byte
	resize [][2]int
	closed bool
}

func (c *recordingChannel) Read([]byte) (int, error) { return 0, io.EOF }
func (c *recordingChannel) Write(p []byte) (int, error) {
	c.wrote = append(c.wrote, p...)
	return len(p), nil
}
func (c *recordingChannel) Close() error { c.closed = true; return nil }
func (c *recordingChannel) Kind() plugin.StreamKind {
	return plugin.StreamTerminal
}

func (c *recordingChannel) Resize(cols, rows int) error {
	c.resize = append(c.resize, [2]int{cols, rows})
	return nil
}

type scriptedClient struct {
	frames [][]byte
}

func (s *scriptedClient) Read(p []byte) (int, error) {
	if len(s.frames) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.frames[0])
	s.frames = s.frames[1:]
	return n, nil
}
func (s *scriptedClient) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (s *scriptedClient) Close() error              { return nil }
func (s *scriptedClient) Context() context.Context  { return context.Background() }

func TestCopyTerminalInputSplitsControlFrames(t *testing.T) {
	ch := &recordingChannel{}
	client := &scriptedClient{frames: [][]byte{
		[]byte("ls"),
		[]byte("\x00{\"type\":\"resize\",\"cols\":120,\"rows\":40}"),
		[]byte("\r"),
		[]byte("\x00{\"type\":\"unknown\"}"), // non-resize control: swallowed, not input
	}}

	if err := plugin.CopyTerminalInput(ch, client); err != io.EOF {
		t.Fatalf("copy: %v", err)
	}
	if got := string(ch.wrote); got != "ls\r" {
		t.Fatalf("keystrokes = %q, want %q (control frames must not leak)", got, "ls\r")
	}
	if len(ch.resize) != 1 || ch.resize[0] != [2]int{120, 40} {
		t.Fatalf("resize = %v, want [[120 40]]", ch.resize)
	}
}

func TestParseResizeControl(t *testing.T) {
	control, ok := plugin.ParseTerminalControl([]byte(`{"type":"resize","cols":1,"rows":2,"theme":"dark"}`))
	if !ok {
		t.Fatal("valid terminal control not parsed")
	}
	if control.Type != "resize" || control.Cols != 1 || control.Rows != 2 || control.Theme != plugin.PanelThemeDark {
		t.Fatalf("terminal control = %#v", control)
	}
	if _, _, ok := plugin.ParseResizeControl([]byte(`{"type":"resize","cols":1,"rows":2,"theme":"dark"}`)); !ok {
		t.Fatal("valid resize not parsed")
	}
	if _, _, ok := plugin.ParseResizeControl([]byte(`{"type":"other"}`)); ok {
		t.Fatal("non-resize parsed as resize")
	}
	if _, _, ok := plugin.ParseResizeControl([]byte("garbage")); ok {
		t.Fatal("garbage parsed as resize")
	}
}

func TestProxyURL(t *testing.T) {
	rc := plugin.NewRequestContext(context.Background(), plugin.User{}, nil, nil, nil, nil).
		WithProxyPrefix("/api/connections/c1/proxy")
	if got := rc.ProxyURL(); got != "/api/connections/c1/proxy/" {
		t.Fatalf("bare proxy url = %q", got)
	}
	if got := rc.ProxyURL("container", "ab/cd", "https:8443"); got != "/api/connections/c1/proxy/container/ab%2Fcd/https:8443/" {
		t.Fatalf("segmented proxy url = %q", got)
	}
	if got := rc.ProxyPrefix(); got != "/api/connections/c1/proxy" {
		t.Fatalf("prefix = %q", got)
	}
	// Trailing slash on the supplied prefix is normalized away.
	rc2 := plugin.NewRequestContext(context.Background(), plugin.User{}, nil, nil, nil, nil).
		WithProxyPrefix("/api/connections/c1/proxy/")
	if got := rc2.ProxyURL(); got != "/api/connections/c1/proxy/" {
		t.Fatalf("normalized proxy url = %q", got)
	}
}
