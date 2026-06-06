// Command agent is shellcn-agent: a plugin-agnostic reverse-tunnel proxy run
// inside a private target. It dials back to the gateway, presents its enrollment
// token, then exposes its declared local target (a TCP address or unix socket)
// over a multiplexed tunnel so the gateway can reach a network it cannot dial.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/hashicorp/yamux"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/telemetry"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		connectURL  string
		token       string
		insecure    bool
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&connectURL, "connect", os.Getenv("SHELLCN_CONNECT_URL"), "gateway agent-connect URL (wss://host/api/agent/connect)")
	flag.StringVar(&token, "token", os.Getenv("SHELLCN_ENROLL_TOKEN"), "enrollment token")
	flag.BoolVar(&insecure, "insecure", os.Getenv("SHELLCN_INSECURE") == "1", "DEVELOPMENT ONLY: allow ws:// and skip TLS verification")
	flag.Parse()

	if showVersion {
		fmt.Printf("%s %s\n", app.AgentBinary, version)
		return
	}

	logger := telemetry.NewLogger(telemetry.LogConfig{Format: telemetry.LogFormatConsole, Output: os.Stderr})

	if connectURL == "" || token == "" {
		logger.Error("missing required config", "connect", connectURL != "", "token", token != "")
		os.Exit(2)
	}

	if err := checkConnectURL(connectURL, insecure); err != nil {
		logger.Error("invalid connect URL", "err", err)
		os.Exit(2)
	}

	if insecure {
		logger.Warn("insecure mode: TLS verification disabled and plaintext ws:// permitted — do not use in production")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	run(ctx, logger, connectURL, token, insecure)
}

// checkConnectURL enforces wss:// (encrypted, CA-validated) unless the operator
// has explicitly opted into insecure mode for development.
func checkConnectURL(connectURL string, insecure bool) error {
	u, err := url.Parse(connectURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "wss":
		return nil
	case "ws":
		if insecure {
			return nil
		}
		return fmt.Errorf("refusing plaintext ws:// without -insecure; use wss://")
	default:
		return fmt.Errorf("connect URL must use wss:// (got %q)", u.Scheme)
	}
}

// errEnrollmentRejected marks a fatal handshake rejection (bad, revoked, or
// never-enrolled expired token): retrying with the same token will not succeed.
var errEnrollmentRejected = errors.New("enrollment rejected by gateway")

// run keeps a tunnel up, reconnecting with backoff until the context is cancelled.
func run(ctx context.Context, logger *slog.Logger, connectURL, token string, insecure bool) {
	backoff := initialReconnectBackoff
	for {
		started := time.Now()
		err := serve(ctx, logger, connectURL, token, insecure)

		backoff = resetReconnectBackoff(backoff, time.Since(started))

		if errors.Is(err, errEnrollmentRejected) {
			logger.Error("enrollment rejected — token is invalid, revoked, or expired before first use; not retrying", "err", err)
			return
		}

		if err != nil && ctx.Err() == nil {
			logger.Warn("tunnel ended, reconnecting", "err", err, "in", backoff)
		}

		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

const (
	initialReconnectBackoff = time.Second
	maxBackoff              = 30 * time.Second
	stableTunnelDuration    = time.Minute
)

func resetReconnectBackoff(backoff, tunnelDuration time.Duration) time.Duration {
	if tunnelDuration >= stableTunnelDuration {
		return initialReconnectBackoff
	}
	return backoff
}

// serve runs a single tunnel lifetime: dial, handshake, then accept + proxy
// multiplexed streams until the tunnel closes.
func serve(ctx context.Context, logger *slog.Logger, connectURL, token string, insecure bool) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	var opts *websocket.DialOptions

	if insecure {
		opts = &websocket.DialOptions{HTTPClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // explicit dev opt-in
		}}
	}

	c, httpResp, err := websocket.Dial(dialCtx, connectURL, opts)
	cancel()

	if httpResp != nil && httpResp.Body != nil {
		defer func() { _ = httpResp.Body.Close() }()
	}

	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	defer func() { _ = c.CloseNow() }()

	hctx, hcancel := context.WithTimeout(ctx, 10*time.Second)
	defer hcancel()

	if err := wsjson.Write(hctx, c, transport.AgentHello{Token: token, Forward: true}); err != nil {
		return fmt.Errorf("handshake write: %w", err)
	}

	var resp transport.AgentConnectResponse
	if err := wsjson.Read(hctx, c, &resp); err != nil {
		return fmt.Errorf("handshake read: %w", err)
	}

	if !resp.OK {
		return fmt.Errorf("%w: %s", errEnrollmentRejected, resp.Error)
	}

	target := resp.Proxy
	logger.Info("tunnel online", "mode", target.Mode, "address", target.Address)

	tunnelCtx, tunnelCancel := context.WithCancel(ctx)
	defer tunnelCancel()
	go func() {
		if err := transport.KeepAliveWebSocket(tunnelCtx, c); err != nil {
			tunnelCancel()
			_ = c.CloseNow()
		}
	}()

	nc := websocket.NetConn(tunnelCtx, c, websocket.MessageBinary)

	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.LogOutput = io.Discard

	sess, err := yamux.Server(nc, cfg)
	if err != nil {
		return fmt.Errorf("yamux: %w", err)
	}

	defer func() { _ = sess.Close() }()

	if target.Mode == transport.AgentModeHostMonitor {
		return serveHostMonitor(ctx, logger, sess)
	}

	if target.Mode == string(transport.AgentModeHTTP) {
		return serveHTTPProxy(ctx, logger, sess, target)
	}

	for {
		stream, err := sess.Accept()
		if err != nil {
			return err
		}
		go proxyStream(logger, stream, target)
	}
}

func serveHostMonitor(ctx context.Context, logger *slog.Logger, sess *yamux.Session) error {
	srv := &http.Server{
		Handler:           hostmonitor.Handler(hostmonitor.NewLocal(hostmonitor.Options{})),
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	logger.Info("host_monitor online")

	if err := srv.Serve(yamuxListener{sess: sess}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// serveHTTPProxy serves the generic L7 mode: a credential-injecting reverse
// proxy over the tunnel, one request per accepted stream. Nothing here is
// protocol-specific.
func serveHTTPProxy(ctx context.Context, logger *slog.Logger, sess *yamux.Session, target transport.AgentProxyTarget) error {
	proxy, err := buildHTTPProxy(logger, target)
	if err != nil {
		return fmt.Errorf("http_proxy: %w", err)
	}

	srv := &http.Server{Handler: proxy, ReadHeaderTimeout: 30 * time.Second}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	logger.Info("http_proxy online", "upstream", target.Address)

	if err := srv.Serve(yamuxListener{sess: sess}); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// buildHTTPProxy builds a reverse proxy to target.Address, injecting an optional
// bearer token from target.TokenFile and verifying TLS with target.CAFile (else
// system roots; no insecure fallback). Fully generic — no domain knowledge.
func buildHTTPProxy(logger *slog.Logger, target transport.AgentProxyTarget) (*httputil.ReverseProxy, error) {
	base := strings.TrimSpace(target.Address)
	if base == "" {
		return nil, errors.New("http_proxy requires an upstream address")
	}

	upstream, err := url.Parse(base)
	if err != nil || upstream.Host == "" {
		return nil, fmt.Errorf("invalid upstream address %q", base)
	}

	// HTTP/1.1 only: connection upgrades (used by streaming endpoints) require
	// HTTP/1.1; the reverse proxy forwards those upgrades transparently.
	tlsConfig := &tls.Config{ServerName: upstream.Hostname(), MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}}
	if caPath := strings.TrimSpace(target.CAFile); caPath != "" {
		pool, err := loadCAPool(caPath)
		if err != nil {
			return nil, fmt.Errorf("load upstream CA: %w", err)
		}
		tlsConfig.RootCAs = pool
	}

	var tokens *tokenSource
	if tokenPath := strings.TrimSpace(target.TokenFile); tokenPath != "" {
		tokens = &tokenSource{path: tokenPath}
	}

	baseTransport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = upstream.Host
			pr.Out.Header.Del("Authorization")
		},
		Transport: bearerTokenTransport{base: baseTransport, tokens: tokens},
		// Flush immediately so server-push streams (watches, logs) aren't buffered.
		FlushInterval: -1,
		ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			logger.Warn("upstream proxy error", "err", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	return rp, nil
}

type bearerTokenTransport struct {
	base   http.RoundTripper
	tokens *tokenSource
}

func (t bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.tokens != nil {
		tok, err := t.tokens.token()
		if err != nil {
			return nil, fmt.Errorf("read bearer token: %w", err)
		}
		if tok != "" {
			req = req.Clone(req.Context())
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
	return t.base.RoundTrip(req)
}

func loadCAPool(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates parsed from %s", path)
	}
	return pool, nil
}

// tokenSource reads a bearer-token file, caching it briefly so a rotated token
// is picked up without a file read on every request.
type tokenSource struct {
	path string
	mu   sync.Mutex
	val  string
	exp  time.Time
}

func (t *tokenSource) token() (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.val != "" && time.Now().Before(t.exp) {
		return t.val, nil
	}

	b, err := os.ReadFile(t.path)
	if err != nil {
		if t.val != "" {
			return t.val, nil
		}
		return "", err
	}

	t.val = strings.TrimSpace(string(b))
	t.exp = time.Now().Add(time.Minute)

	return t.val, nil
}

// yamuxListener adapts a yamux session to net.Listener so an http.Server can
// serve a request per accepted stream.
type yamuxListener struct{ sess *yamux.Session }

func (l yamuxListener) Accept() (net.Conn, error) { return l.sess.Accept() }
func (l yamuxListener) Close() error              { return l.sess.Close() }
func (l yamuxListener) Addr() net.Addr            { return l.sess.Addr() }

// proxyStream pipes one gateway stream to the declared local target.
// streamTarget resolves where a stream is proxied: the per-stream preamble in
// forward mode, else the agent's single declared Address.
func streamTarget(logger *slog.Logger, stream net.Conn, target transport.AgentProxyTarget) (network, address string) {
	if target.Forward {
		netw, addr, err := transport.ReadStreamTarget(stream)
		if err != nil {
			logger.Warn("read stream target failed", "err", err)
			return "", ""
		}
		network, address = netw, addr
	} else {
		network, address = modeNetwork(target.Mode), target.Address
	}
	if network != "tcp" && network != "unix" {
		logger.Warn("refusing unsupported proxy network", "network", network)
		return "", ""
	}
	return network, address
}

func modeNetwork(mode string) string {
	switch mode {
	case transport.AgentModeTCP:
		return "tcp"
	case transport.AgentModeUnix:
		return "unix"
	default:
		return ""
	}
}

func proxyStream(logger *slog.Logger, stream net.Conn, target transport.AgentProxyTarget) {
	defer func() { _ = stream.Close() }()

	network, address := streamTarget(logger, stream, target)
	if network == "" {
		return
	}

	up, err := net.DialTimeout(network, address, 10*time.Second)
	if err != nil {
		logger.Warn("dial target failed", "address", address, "err", err)
		return
	}

	defer func() { _ = up.Close() }()

	done := make(chan error, 2)

	go func() { _, e := io.Copy(up, stream); done <- e }()
	go func() { _, e := io.Copy(stream, up); done <- e }()

	<-done
}
