package kubernetes

import (
	"context"
	"io"
	"net"
	"sync"
)

// loopbackBridge exposes the agent tunnel as a local TCP endpoint. client-go's
// exec/port-forward upgraders dial their host directly and ignore a custom
// dialer (kubernetes #129915), so for agent transport we front the tunnel with a
// loopback listener: each accepted connection is piped to a fresh tunnel stream,
// which the agent's reverse proxy terminates and re-originates (with the target's
// credentials) to the API server. It listens on 127.0.0.1 only, for the
// session's lifetime.
type loopbackBridge struct {
	ln   net.Listener
	dial func(context.Context) (net.Conn, error)
	once sync.Once
}

func newLoopbackBridge(dial func(context.Context) (net.Conn, error)) (*loopbackBridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &loopbackBridge{ln: ln, dial: dial}
	go b.serve()
	return b, nil
}

func (b *loopbackBridge) host() string { return "http://" + b.ln.Addr().String() }

func (b *loopbackBridge) serve() {
	for {
		c, err := b.ln.Accept()
		if err != nil {
			return
		}
		go b.pipe(c)
	}
}

func (b *loopbackBridge) pipe(local net.Conn) {
	defer func() { _ = local.Close() }()
	up, err := b.dial(context.Background())
	if err != nil {
		return
	}
	defer func() { _ = up.Close() }()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, up); done <- struct{}{} }()
	<-done
}

func (b *loopbackBridge) Close() error {
	var err error
	b.once.Do(func() { err = b.ln.Close() })
	return err
}
