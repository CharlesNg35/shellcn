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
	ln   net.Listener
	dial func(context.Context) (net.Conn, error)
	once sync.Once
}

// New starts a bridge that pipes each accepted connection to dial's tunnel stream.
func New(dial func(context.Context) (net.Conn, error)) (*Bridge, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Bridge{ln: ln, dial: dial}
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
		go b.pipe(c)
	}
}

func (b *Bridge) pipe(local net.Conn) {
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

func (b *Bridge) Close() error {
	var err error
	b.once.Do(func() { err = b.ln.Close() })
	return err
}
