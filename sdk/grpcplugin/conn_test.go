package grpcplugin

import (
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin/canvas"
)

// memStream is one end of an in-memory Conn.Pipe: what the peer sends arrives
// verbatim, one Recv per Send, so a test sees the same chunking gRPC gives.
type memStream struct {
	out     chan []byte
	in      chan []byte
	once    sync.Once
	closing chan struct{}
}

func (m *memStream) Send(chunk *pluginv1.Chunk) error {
	select {
	case m.out <- append([]byte(nil), chunk.GetData()...):
		return nil
	case <-m.closing:
		return io.ErrClosedPipe
	}
}

func (m *memStream) Recv() (*pluginv1.Chunk, error) {
	data, ok := <-m.in
	if !ok {
		return nil, io.EOF
	}
	return &pluginv1.Chunk{Data: data}, nil
}

func (m *memStream) CloseSend() error {
	m.once.Do(func() {
		close(m.closing)
		close(m.out)
	})
	return nil
}

func memConnPair() (net.Conn, net.Conn) {
	a := &memStream{out: make(chan []byte, 8), closing: make(chan struct{})}
	b := &memStream{out: make(chan []byte, 8), closing: make(chan struct{})}
	a.in, b.in = b.out, a.out
	return newStreamConn(a, nil), newStreamConn(b, nil)
}

// boundaryWriter stands in for the WebSocket relay: every Write becomes one
// browser message.
type boundaryWriter struct {
	messages [][]byte
}

func (w *boundaryWriter) Write(p []byte) (int, error) {
	w.messages = append(w.messages, append([]byte(nil), p...))
	return len(p), nil
}

func largeCanvasFrame(points int) canvas.Frame {
	commands := make([]canvas.Command, 0, points+1)
	commands = append(commands, canvas.Clear{Color: "#020617"})
	for i := 0; i < points; i++ {
		commands = append(commands, canvas.Circle{
			Paint:  canvas.Paint{Fill: "rgba(56,189,248,0.45)"},
			X:      float64(i) * 1.0 / 3.0,
			Y:      float64(i) * 2.0 / 3.0,
			Radius: 3.5,
		})
	}
	return canvas.Frame{Commands: commands}
}

// Bridge relays a stream with io.Copy, so a canvas frame larger than its 32 KiB
// buffer used to reach the browser as mid-JSON fragments, which the canvas panel
// rejects with "Invalid canvas frame". Each plugin write must stay one message.
func TestStreamConnCopyPreservesWriteBoundaries(t *testing.T) {
	pluginSide, hostSide := memConnPair()
	frame := largeCanvasFrame(512)

	go func() {
		for i := 0; i < 2; i++ {
			if err := canvas.WriteFrame(pluginSide, frame); err != nil {
				t.Error(err)
				break
			}
		}
		_ = pluginSide.Close()
	}()

	relay := &boundaryWriter{}
	if _, err := io.Copy(relay, hostSide); err != nil {
		t.Fatalf("relay copy: %v", err)
	}

	if len(relay.messages) != 2 {
		t.Fatalf("expected one message per frame, got %d", len(relay.messages))
	}
	for i, msg := range relay.messages {
		if len(msg) <= 32*1024 {
			t.Fatalf("message %d is too small (%d bytes) to prove the frame survived un-cut", i, len(msg))
		}
		var decoded map[string]any
		if err := json.Unmarshal(msg, &decoded); err != nil {
			t.Fatalf("message %d is not a complete canvas frame: %v", i, err)
		}
	}
}

func TestStreamConnReadSplitsChunksAcrossBuffers(t *testing.T) {
	pluginSide, hostSide := memConnPair()
	go func() {
		if _, err := pluginSide.Write([]byte("hello world")); err != nil {
			t.Error(err)
		}
		_ = pluginSide.Close()
	}()

	got := make([]byte, 0, 11)
	buf := make([]byte, 4)
	for {
		n, err := hostSide.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	if string(got) != "hello world" {
		t.Fatalf("short reads lost bytes: %q", got)
	}
}
