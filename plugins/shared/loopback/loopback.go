// Package loopback fronts an agent tunnel as a local TCP endpoint. Clients that
// bypass a custom dialer — Docker's exec hijack, client-go's SPDY/WebSocket
// upgraders — need a real socket; each accepted connection is piped to a fresh
// tunnel stream. It listens on 127.0.0.1 only, for the session's lifetime.
package loopback

import (
	"context"
	"io"
	"net"
	"sync"
)

type Bridge struct {
	ctx    context.Context
	cancel context.CancelFunc
	ln     net.Listener
	dial   func(context.Context) (net.Conn, error)
	once   sync.Once
	mu     sync.Mutex
	conns  map[net.Conn]struct{}
}

// New starts a bridge that pipes each accepted connection to dial's tunnel stream.
func New(dial func(context.Context) (net.Conn, error)) (*Bridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bridge{ctx: ctx, cancel: cancel, ln: ln, dial: dial, conns: map[net.Conn]struct{}{}}
	go b.serve()
	return b, nil
}

// Addr is the bridge's host:port; Host is it as an http:// URL.
func (b *Bridge) Addr() string { return b.ln.Addr().String() }
func (b *Bridge) Host() string { return "http://" + b.ln.Addr().String() }

func (b *Bridge) serve() {
	for {
		c, err := b.ln.Accept()
		if err != nil {
			return
		}
		b.track(c)
		go b.pipe(c)
	}
}

func (b *Bridge) pipe(local net.Conn) {
	defer b.closeTracked(local)
	up, err := b.dial(b.ctx)
	if err != nil {
		return
	}
	b.track(up)
	defer b.closeTracked(up)
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, up); done <- struct{}{} }()
	<-done
}

func (b *Bridge) track(c net.Conn) {
	b.mu.Lock()
	b.conns[c] = struct{}{}
	b.mu.Unlock()
}

func (b *Bridge) closeTracked(c net.Conn) {
	b.mu.Lock()
	delete(b.conns, c)
	b.mu.Unlock()
	_ = c.Close()
}

func (b *Bridge) Close() error {
	var err error
	b.once.Do(func() {
		b.cancel()
		err = b.ln.Close()
		b.mu.Lock()
		conns := make([]net.Conn, 0, len(b.conns))
		for c := range b.conns {
			conns = append(conns, c)
		}
		b.mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return err
}
