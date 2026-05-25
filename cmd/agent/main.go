// Command agent is shellcn-agent: a plugin-agnostic reverse-tunnel proxy run
// inside a private target. It dials back to the gateway, presents its enrollment
// token, then exposes its declared local target (a TCP address or unix socket)
// over a multiplexed tunnel so the gateway can reach a network it cannot dial.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/hashicorp/yamux"

	"github.com/charlesng/shellcn/internal/transport"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		connectURL  string
		token       string
		target      string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&connectURL, "connect", os.Getenv("SHELLCN_CONNECT_URL"), "gateway agent-connect URL (wss://host/api/agent/connect)")
	flag.StringVar(&token, "token", os.Getenv("SHELLCN_ENROLL_TOKEN"), "enrollment token")
	flag.StringVar(&target, "target", os.Getenv("SHELLCN_TARGET"), "override the local target address the gateway told us to proxy")
	flag.Parse()

	if showVersion {
		fmt.Printf("shellcn-agent %s\n", version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if connectURL == "" || token == "" {
		logger.Error("missing required config", "connect", connectURL != "", "token", token != "")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	run(ctx, logger, connectURL, token, target)
}

// run keeps a tunnel up, reconnecting with backoff until the context is cancelled.
func run(ctx context.Context, logger *slog.Logger, connectURL, token, targetOverride string) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if err := serve(ctx, logger, connectURL, token, targetOverride); err != nil && ctx.Err() == nil {
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

// serve runs a single tunnel lifetime: dial, handshake, then accept + proxy
// multiplexed streams until the tunnel closes.
func serve(ctx context.Context, logger *slog.Logger, connectURL, token, targetOverride string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	c, _, err := websocket.Dial(dialCtx, connectURL, nil)
	cancel()
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = c.CloseNow() }()

	hctx, hcancel := context.WithTimeout(ctx, 10*time.Second)
	defer hcancel()
	if err := wsjson.Write(hctx, c, transport.AgentHello{Token: token}); err != nil {
		return fmt.Errorf("handshake write: %w", err)
	}
	var resp transport.AgentConnectResponse
	if err := wsjson.Read(hctx, c, &resp); err != nil {
		return fmt.Errorf("handshake read: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("gateway rejected enrollment: %s", resp.Error)
	}

	target := resp.Proxy
	if targetOverride != "" {
		target.Address = targetOverride
	}
	logger.Info("tunnel online", "mode", target.Mode, "address", target.Address)

	nc := websocket.NetConn(ctx, c, websocket.MessageBinary)
	cfg := yamux.DefaultConfig()
	cfg.EnableKeepAlive = true
	cfg.LogOutput = io.Discard
	sess, err := yamux.Server(nc, cfg)
	if err != nil {
		return fmt.Errorf("yamux: %w", err)
	}
	defer func() { _ = sess.Close() }()

	for {
		stream, err := sess.Accept()
		if err != nil {
			return err
		}
		go proxyStream(logger, stream, target)
	}
}

// proxyStream pipes one gateway stream to the declared local target.
func proxyStream(logger *slog.Logger, stream net.Conn, target transport.AgentProxyTarget) {
	defer func() { _ = stream.Close() }()

	network := "tcp"
	if target.Mode == "unix" {
		network = "unix"
	}
	up, err := net.DialTimeout(network, target.Address, 10*time.Second)
	if err != nil {
		logger.Warn("dial target failed", "address", target.Address, "err", err)
		return
	}
	defer func() { _ = up.Close() }()

	done := make(chan error, 2)
	go func() { _, e := io.Copy(up, stream); done <- e }()
	go func() { _, e := io.Copy(stream, up); done <- e }()
	<-done
}
