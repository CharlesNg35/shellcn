// Package dockerengine is the shared runtime for Docker-API protocols. The
// docker, swarm, and podman plugins all talk to a Docker-compatible daemon over
// the moby client, so the session, streaming channels, row mapping, pagination,
// raw-API passthrough, and generic route handlers live here once. Each plugin
// supplies only its manifest and the route wiring (IDs, permissions, paths).
package dockerengine

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type endpoint struct {
	network string
	address string
}

// Session is a live connection to one Docker-compatible daemon. It is created
// per connection and holds all per-connection state; plugins stay stateless.
type Session struct {
	cli      *dockerclient.Client
	http     *http.Client
	endpoint endpoint
}

// Client exposes the moby client so plugin-specific handlers (swarm services,
// nodes, tasks; podman pods) can issue calls the shared handlers don't cover.
func (s *Session) Client() *dockerclient.Client { return s.cli }

// HTTPClient exposes the daemon-dialing HTTP client for raw API access (the raw
// API panel, and podman's libpod-only endpoints).
func (s *Session) HTTPClient() *http.Client { return s.http }

// Connect dials the daemon for cfg's transport. defaultSocket is the unix socket
// used when none is configured (e.g. /var/run/docker.sock, /run/podman/podman.sock).
func Connect(ctx context.Context, cfg plugin.ConnectConfig, defaultSocket string) (plugin.Session, error) {
	ep, err := dockerEndpoint(cfg, defaultSocket)
	if err != nil {
		return nil, err
	}
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return cfg.Net.DialContext(ctx, ep.network, ep.address)
	}
	cli, err := dockerclient.New(
		dockerclient.WithHost("http://docker"),
		dockerclient.WithDialContext(dial),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: create docker client: %v", plugin.ErrUnavailable, err)
	}
	s := &Session{
		cli:      cli,
		endpoint: ep,
		http: &http.Client{Transport: &http.Transport{
			DialContext: dial,
		}},
	}
	if err := s.HealthCheck(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func dockerEndpoint(cfg plugin.ConnectConfig, defaultSocket string) (endpoint, error) {
	mode := cfg.String("endpoint_type")
	if mode == "" {
		mode = "unix"
	}
	switch mode {
	case "unix":
		socket := cfg.String("socket_path")
		if socket == "" {
			socket = defaultSocket
		}
		if !strings.HasPrefix(socket, "/") {
			return endpoint{}, fmt.Errorf("%w: docker socket path must be absolute", plugin.ErrInvalidInput)
		}
		return endpoint{network: "unix", address: socket}, nil
	case "tcp":
		host := strings.TrimSpace(cfg.String("host"))
		if host == "" {
			return endpoint{}, fmt.Errorf("%w: docker host is required", plugin.ErrInvalidInput)
		}
		port, ok := cfg.Int("port")
		if !ok || port <= 0 || port > 65535 {
			return endpoint{}, fmt.Errorf("%w: docker port must be between 1 and 65535", plugin.ErrInvalidInput)
		}
		return endpoint{network: "tcp", address: net.JoinHostPort(host, strconv.Itoa(port))}, nil
	default:
		return endpoint{}, fmt.Errorf("%w: unsupported docker endpoint type %q", plugin.ErrInvalidInput, mode)
	}
}

// Unwrap recovers the engine Session from a plugin.Session, transparently
// looking through a wrapper (e.g. the recording layer) that exposes Session().
func Unwrap(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	type sessionGetter interface {
		Session() plugin.Session
	}
	if h, ok := sess.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: Docker session unavailable", plugin.ErrUnavailable)
}

func (s *Session) HealthCheck(ctx context.Context) error {
	// Negotiate the API version up front so a daemon below the client's minimum
	// (notably Podman's older Docker-compat version) fails here with a clear
	// error instead of lazily on the first request with a raw HTTP 400.
	if _, err := s.cli.Ping(ctx, dockerclient.PingOptions{NegotiateAPIVersion: true}); err != nil {
		return fmt.Errorf("%w: docker ping: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) Close() error {
	return s.cli.Close()
}

func (s *Session) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	switch req.Kind {
	case plugin.StreamLogs:
		return s.openLogs(ctx, req.Params)
	case plugin.StreamTerminal:
		return s.openExec(ctx, req.Params)
	default:
		return nil, plugin.ErrNotSupported
	}
}

type logsChannel struct {
	io.Reader
	close func() error
	once  sync.Once
}

func (c *logsChannel) Kind() plugin.StreamKind { return plugin.StreamLogs }

func (c *logsChannel) Write(p []byte) (int, error) { return len(p), nil }

func (c *logsChannel) Close() error {
	var err error
	c.once.Do(func() { err = c.close() })
	return err
}

type execChannel struct {
	cli    *dockerclient.Client
	execID string
	resp   dockerclient.HijackedResponse
	once   sync.Once
}

func (c *execChannel) Kind() plugin.StreamKind { return plugin.StreamTerminal }

func (c *execChannel) Read(p []byte) (int, error) {
	return c.resp.Reader.Read(p)
}

func (c *execChannel) Write(p []byte) (int, error) {
	return c.resp.Conn.Write(p)
}

func (c *execChannel) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	_, err := c.cli.ExecResize(context.Background(), c.execID, dockerclient.ExecResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})
	return err
}

func (c *execChannel) Close() error {
	c.once.Do(func() { c.resp.Close() })
	return nil
}

func rawAPIPath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("%w: Docker API path is required", plugin.ErrInvalidInput)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: invalid Docker API path", plugin.ErrInvalidInput)
	}
	if u.IsAbs() {
		if u.Host != "docker" {
			return "", fmt.Errorf("%w: Docker API requests must target the docker daemon", plugin.ErrInvalidInput)
		}
		return u.RequestURI(), nil
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("%w: Docker API path must start with /", plugin.ErrInvalidInput)
	}
	return raw, nil
}
