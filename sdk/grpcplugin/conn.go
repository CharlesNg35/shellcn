package grpcplugin

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
)

// chunkStream is the common surface of the Conn.Pipe client and server streams.
type chunkStream interface {
	Send(*pluginv1.Chunk) error
	Recv() (*pluginv1.Chunk, error)
}

// streamConn presents a Conn.Pipe byte stream as a net.Conn. Deadlines are
// no-ops: the stream's lifetime is governed by its context and Close.
type streamConn struct {
	stream    chunkStream
	onClose   func()
	closeOnce sync.Once

	rmu sync.Mutex
	buf []byte
	wmu sync.Mutex
}

func newStreamConn(stream chunkStream, onClose func()) *streamConn {
	return &streamConn{stream: stream, onClose: onClose}
}

func (c *streamConn) Read(p []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	if len(c.buf) == 0 {
		chunk, err := c.stream.Recv()
		if err != nil {
			return 0, err
		}
		c.buf = chunk.GetData()
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

func (c *streamConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := c.stream.Send(&pluginv1.Chunk{Data: append([]byte(nil), p...)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *streamConn) Close() error {
	c.closeOnce.Do(func() {
		if cs, ok := c.stream.(interface{ CloseSend() error }); ok {
			_ = cs.CloseSend()
		}
		if c.onClose != nil {
			c.onClose()
		}
	})
	return nil
}

func (*streamConn) LocalAddr() net.Addr              { return pipeAddr{} }
func (*streamConn) RemoteAddr() net.Addr             { return pipeAddr{} }
func (*streamConn) SetDeadline(time.Time) error      { return nil }
func (*streamConn) SetReadDeadline(time.Time) error  { return nil }
func (*streamConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "shellcn-broker" }
func (pipeAddr) String() string  { return "broker" }

// pipeServer adapts a per-conn handler to the Conn service: handle runs once per
// brokered Pipe with the stream presented as a net.Conn and the stream context.
type pipeServer struct {
	pluginv1.UnimplementedConnServer
	handle func(ctx context.Context, conn net.Conn) error
}

// NewPipeServer serves handle over a brokered Conn.Pipe (one call per conn).
func NewPipeServer(handle func(ctx context.Context, conn net.Conn) error) pluginv1.ConnServer {
	return &pipeServer{handle: handle}
}

func (p *pipeServer) Pipe(stream pluginv1.Conn_PipeServer) error {
	return p.handle(stream.Context(), newStreamConn(stream, nil))
}

// NewConnBridge serves a real conn over a brokered Conn.Pipe stream.
func NewConnBridge(target net.Conn) pluginv1.ConnServer {
	return NewPipeServer(func(_ context.Context, conn net.Conn) error {
		Bridge(target, conn)
		return nil
	})
}

// ServeConn registers srv on a fresh brokered id and returns it for the peer to Dial.
func ServeConn(broker *goplugin.GRPCBroker, srv pluginv1.ConnServer) uint32 {
	id := broker.NextId()
	go broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		s := grpc.NewServer(opts...)
		pluginv1.RegisterConnServer(s, srv)
		return s
	})
	return id
}

// DialConn dials a brokered Conn served under id and presents it as a net.Conn.
func DialConn(broker *goplugin.GRPCBroker, id uint32) (net.Conn, error) {
	cc, err := broker.Dial(id)
	if err != nil {
		return nil, err
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	stream, err := pluginv1.NewConnClient(cc).Pipe(streamCtx)
	if err != nil {
		cancel()
		_ = cc.Close()
		return nil, err
	}
	return newStreamConn(stream, func() { cancel(); _ = cc.Close() }), nil
}

// Bridge copies bytes both ways between two conns until either side ends, then
// closes both (closes are idempotent for brokered conns).
func Bridge(a, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}
