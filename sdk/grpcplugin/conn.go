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

	rmu      sync.Mutex
	readOnce sync.Once
	readCh   chan readResult
	buf      []byte
	readErr  error

	deadlineMu sync.Mutex
	readUntil  time.Time
	writeUntil time.Time
	readWake   chan struct{}
	wmu        sync.Mutex
}

func newStreamConn(stream chunkStream, onClose func()) *streamConn {
	return &streamConn{stream: stream, onClose: onClose, readCh: make(chan readResult, 1), readWake: make(chan struct{})}
}

func (c *streamConn) Read(p []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	if len(c.buf) == 0 {
		if c.readErr != nil {
			return 0, c.readErr
		}
		c.readOnce.Do(func() {
			go c.recvLoop()
		})
		for len(c.buf) == 0 {
			deadline, wake := c.readDeadline()
			if timeout := deadlineTimeout(deadline); timeout <= 0 && !deadline.IsZero() {
				return 0, timeoutError{}
			} else if deadline.IsZero() {
				select {
				case got := <-c.readCh:
					if got.err != nil {
						c.readErr = got.err
						return 0, got.err
					}
					c.buf = got.data
				case <-wake:
					continue
				}
			} else {
				timer := time.NewTimer(timeout)
				select {
				case got := <-c.readCh:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					if got.err != nil {
						c.readErr = got.err
						return 0, got.err
					}
					c.buf = got.data
				case <-timer.C:
					return 0, timeoutError{}
				case <-wake:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					continue
				}
			}
		}
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

func (c *streamConn) Write(p []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if deadline, ok := c.writeDeadline(); ok && !deadline.After(time.Now()) {
		return 0, timeoutError{}
	}
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

func (*streamConn) LocalAddr() net.Addr  { return pipeAddr{} }
func (*streamConn) RemoteAddr() net.Addr { return pipeAddr{} }
func (c *streamConn) SetDeadline(t time.Time) error {
	c.setReadDeadline(t)
	c.setWriteDeadline(t)
	return nil
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	c.setReadDeadline(t)
	return nil
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	c.setWriteDeadline(t)
	return nil
}

type readResult struct {
	data []byte
	err  error
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func (c *streamConn) recvLoop() {
	for {
		chunk, err := c.stream.Recv()
		if err != nil {
			c.readCh <- readResult{err: err}
			return
		}
		c.readCh <- readResult{data: chunk.GetData()}
	}
}

func (c *streamConn) readDeadline() (time.Time, <-chan struct{}) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	return c.readUntil, c.readWake
}

func (c *streamConn) writeDeadline() (time.Time, bool) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	return c.writeUntil, !c.writeUntil.IsZero()
}

func (c *streamConn) setReadDeadline(t time.Time) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.readUntil = t
	close(c.readWake)
	c.readWake = make(chan struct{})
}

func (c *streamConn) setWriteDeadline(t time.Time) {
	c.deadlineMu.Lock()
	defer c.deadlineMu.Unlock()
	c.writeUntil = t
}

func deadlineTimeout(deadline time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	return time.Until(deadline)
}

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
