package main

import (
	"encoding/pem"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/transport"
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

func TestTokenSourceReadsAndCaches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  tok-1\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	ts := &tokenSource{path: path}
	got, err := ts.token()
	if err != nil || got != "tok-1" {
		t.Fatalf("token() = %q, %v; want tok-1", got, err)
	}
	// A rewrite within the cache window keeps the old value.
	if err := os.WriteFile(path, []byte("tok-2"), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if got, _ := ts.token(); got != "tok-1" {
		t.Fatalf("cached token() = %q; want tok-1", got)
	}
	// Expiring the cache picks up the rotated token.
	ts.exp = time.Now().Add(-time.Second)
	if got, _ := ts.token(); got != "tok-2" {
		t.Fatalf("refreshed token() = %q; want tok-2", got)
	}
}

func TestBuildAPIProxyInjectsCredentials(t *testing.T) {
	var gotAuth, gotPath string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"kind":"NamespaceList"}`)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw})
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("sa-token-xyz"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}

	proxy, err := buildHTTPProxy(slog.New(slog.NewTextHandler(io.Discard, nil)),
		transport.AgentProxyTarget{Mode: transport.AgentModeHTTP, Address: upstream.URL, TokenFile: tokenPath, CAFile: caPath})
	if err != nil {
		t.Fatalf("buildHTTPProxy: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+app.AgentInternalHost+"/api/v1/namespaces", nil)
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer sa-token-xyz" {
		t.Errorf("upstream Authorization = %q; want injected bearer token", gotAuth)
	}
	if gotPath != "/api/v1/namespaces" {
		t.Errorf("upstream path = %q; want /api/v1/namespaces", gotPath)
	}
}

func TestBuildHTTPProxyRequiresAddress(t *testing.T) {
	if _, err := buildHTTPProxy(slog.New(slog.NewTextHandler(io.Discard, nil)),
		transport.AgentProxyTarget{Mode: transport.AgentModeHTTP}); err == nil {
		t.Fatal("buildHTTPProxy must require an upstream address")
	}
}

func TestBuildHTTPProxyRejectsBadCA(t *testing.T) {
	if _, err := buildHTTPProxy(slog.New(slog.NewTextHandler(io.Discard, nil)),
		transport.AgentProxyTarget{Mode: transport.AgentModeHTTP, Address: "https://example.internal:443", CAFile: filepath.Join(t.TempDir(), "missing-ca")}); err == nil {
		t.Fatal("buildHTTPProxy must fail when the declared CA file is unreadable (no insecure fallback)")
	}
}

func TestResetReconnectBackoffAfterStableTunnel(t *testing.T) {
	if got := resetReconnectBackoff(30*time.Second, stableTunnelDuration-time.Nanosecond); got != 30*time.Second {
		t.Fatalf("short tunnel reset = %s, want existing backoff", got)
	}
	if got := resetReconnectBackoff(30*time.Second, stableTunnelDuration); got != initialReconnectBackoff {
		t.Fatalf("stable tunnel reset = %s, want %s", got, initialReconnectBackoff)
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
