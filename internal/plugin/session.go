package plugin

import (
	"context"
	"io"
	"net"
	"net/http"
)

// NetTransport exposes the upstream at the layer the protocol needs. The core
// builds it wired for the connection's mode; the plugin picks a layer and never
// branches on direct-vs-agent.
type NetTransport interface {
	// L4 socket/TCP protocols. Identical for direct and agent transport.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	// L7 clients that need an injected HTTP transport. ok=false unless an L7
	// agent mode is in use.
	HTTP() (baseURL string, rt http.RoundTripper, ok bool)
}

// ConnectConfig is built by the core: decrypted config + a transport wired for
// the connection's mode. The plugin uses the layer its client needs.
type ConnectConfig struct {
	ConnectionID string
	Transport    Transport
	Config       map[string]any
	Net          NetTransport
}

// String returns a typed config value, or "" if absent/not a string.
func (c ConnectConfig) String(key string) string {
	if v, ok := c.Config[key].(string); ok {
		return v
	}
	return ""
}

// Int returns a typed config value; JSON numbers decode to float64, handled here.
func (c ConnectConfig) Int(key string) (int, bool) {
	switch v := c.Config[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// ChannelRequest opens a tracked upstream stream within a session.
type ChannelRequest struct {
	Kind   StreamKind
	Params map[string]string
}

// Channel is one tracked upstream stream.
type Channel interface {
	io.ReadWriteCloser
	Kind() StreamKind
}

// ClientStream is the browser side of a WS pipe handed to a StreamHandler.
type ClientStream interface {
	io.ReadWriteCloser
	// Context is closed when the client disconnects.
	Context() context.Context
}

// Session is a live, authenticated runtime for one connection. It holds all
// per-connection state; the Plugin struct holds none.
type Session interface {
	HealthCheck(ctx context.Context) error
	OpenChannel(ctx context.Context, req ChannelRequest) (Channel, error)
	Close() error
}

// Plugin is a stateless, compiled-in singleton. It DECLARES (Manifest), exposes
// typed ROUTES (handlers), and CONNECTS (returns a Session holding all
// per-connection state). One instance serves every connection.
type Plugin interface {
	Manifest() Manifest
	Routes() []Route
	Connect(ctx context.Context, cfg ConnectConfig) (Session, error)
}

// HealthChecker is an optional plugin capability surfaced on the status endpoint.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// HTTPProxy is an optional Session capability: it reverse-proxies a browser
// request to an upstream the session can reach (e.g. a web service behind the
// connection's network). The core authenticates + authorizes the user and
// strips the connection-proxy route prefix; r.URL.Path is the remaining
// plugin-defined target path. The session maps it to its upstream and streams
// the response (supporting redirects, assets, and WebSocket upgrades). It enables
// generated "open in browser" links without any protocol-specific core code.
type HTTPProxy interface {
	ServeHTTPProxy(w http.ResponseWriter, r *http.Request)
}
