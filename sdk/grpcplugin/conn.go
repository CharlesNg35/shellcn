package grpcplugin

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	pluginv1 "github.com/charlesng35/shellcn/sdk/gen/shellcn/plugin/v1"
)

// chunkStream is the common surface of the Conn.Pipe client and server streams.
type chunkStream interface {
	Send(*pluginv1.Chunk) error
	Recv() (*pluginv1.Chunk, error)
}

// streamConn presents a Conn.Pipe byte stream as a net.Conn. Deadlines are
// no-ops: the stream's lifetime is governed by its context and Close.
type streamConn struct {
	stream chunkStream
	cancel context.CancelFunc

	rmu sync.Mutex
	buf []byte
	wmu sync.Mutex
}

func newStreamConn(stream chunkStream, cancel context.CancelFunc) *streamConn {
	return &streamConn{stream: stream, cancel: cancel}
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
	if cs, ok := c.stream.(interface{ CloseSend() error }); ok {
		_ = cs.CloseSend()
	}
	if c.cancel != nil {
		c.cancel()
	}
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

// connBridge serves Conn.Pipe by copying the stream to and from a real conn.
type connBridge struct {
	pluginv1.UnimplementedConnServer
	target net.Conn
}

// NewConnBridge serves the target conn over a brokered Conn.Pipe stream.
func NewConnBridge(target net.Conn) pluginv1.ConnServer {
	return &connBridge{target: target}
}

func (b *connBridge) Pipe(stream pluginv1.Conn_PipeServer) error {
	bridge(b.target, newStreamConn(stream, nil))
	return nil
}

// bridge copies bytes both ways between two conns until either side ends.
func bridge(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
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
