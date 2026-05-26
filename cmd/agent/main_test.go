package main

import (
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesng/shellcn/internal/transport"
)

func TestProxyStreamRefusesUnsupportedMode(t *testing.T) {
	a, b := net.Pipe()
	defer func() { _ = b.Close() }()
	done := make(chan struct{})
	go func() {
		proxyStream(slog.Default(), a, transport.AgentProxyTarget{Mode: "http", Address: "127.0.0.1:1"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("proxyStream did not return for unsupported mode")
	}
	if _, err := b.Write([]byte("x")); err == nil {
		t.Fatal("unsupported mode left stream writable")
	}
}

func TestProxyStreamTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = io.Copy(conn, conn)
	}()

	a, b := net.Pipe()
	defer func() { _ = b.Close() }()
	go proxyStream(slog.Default(), a, transport.AgentProxyTarget{Mode: "tcp", Address: ln.Addr().String()})

	_ = b.SetDeadline(time.Now().Add(time.Second))
	if _, err := b.Write([]byte("echo")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(b, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "echo" {
		t.Fatalf("echo = %q", buf)
	}
}

func TestProxyStreamUnix(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "target.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = io.Copy(conn, conn)
	}()

	a, b := net.Pipe()
	defer func() { _ = b.Close() }()
	go proxyStream(slog.Default(), a, transport.AgentProxyTarget{Mode: "unix", Address: socket})

	_ = b.SetDeadline(time.Now().Add(time.Second))
	if _, err := b.Write([]byte("unix")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(b, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "unix" {
		t.Fatalf("echo = %q", buf)
	}
}
