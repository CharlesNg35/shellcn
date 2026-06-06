package plugin

import (
	"context"
	"io"
	"net"
	"net/http"
)

// NetTransport exposes the upstream at the layer the protocol needs.
type NetTransport interface {
	// L4 socket/TCP protocols.
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	// L7 clients that need an injected HTTP transport.
	HTTP() (baseURL string, rt http.RoundTripper, ok bool)
}

// ConnectConfig is the decrypted config plus core-built transport.
type ConnectConfig struct {
	ConnectionID string
	Transport    Transport
	Config       map[string]any
	Net          NetTransport
	Storage      Storage
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

// Plugin is a stateless, compiled-in protocol implementation.
type Plugin interface {
	Manifest() Manifest
	Routes() []Route
	Connect(ctx context.Context, cfg ConnectConfig) (Session, error)
}

// HTTPProxy is an optional Session capability for browser-accessible upstreams.
type HTTPProxy interface {
	ServeHTTPProxy(w http.ResponseWriter, r *http.Request)
}
