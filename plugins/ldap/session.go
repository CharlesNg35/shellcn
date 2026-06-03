package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Session struct {
	conn   *ldapv3.Conn
	opts   options
	closed atomic.Bool
}

func connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := dial(ctx, opts, cfg.Net)
	if err != nil {
		return nil, err
	}
	s := &Session{conn: conn, opts: opts}
	if s.opts.BaseDN == "" {
		base, err := s.discoverBaseDN()
		if err != nil {
			_ = s.Close()
			return nil, err
		}
		s.opts.BaseDN = base
	}
	if err := s.HealthCheck(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func dial(ctx context.Context, opts options, netTransport plugin.NetTransport) (*ldapv3.Conn, error) {
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	raw, err := netTransport.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: dial LDAP: %v", plugin.ErrUnavailable, err)
	}
	isTLS := false
	if opts.Encryption == encLDAPS {
		tlsCfg, err := opts.tlsConfig()
		if err != nil {
			_ = raw.Close()
			return nil, err
		}
		tlsConn := tls.Client(raw, tlsCfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("%w: TLS handshake: %v", plugin.ErrUnavailable, err)
		}
		raw, isTLS = tlsConn, true
	}
	conn := ldapv3.NewConn(raw, isTLS)
	conn.Start()
	conn.SetTimeout(opts.Timeout)
	if opts.Encryption == encStartTLS {
		tlsCfg, err := opts.tlsConfig()
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		if err := conn.StartTLS(tlsCfg); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("%w: StartTLS: %v", plugin.ErrUnavailable, err)
		}
	}
	if err := bind(conn, opts); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func bind(conn *ldapv3.Conn, opts options) error {
	if opts.AuthMode == authAnonymous {
		return nil
	}
	if err := conn.Bind(opts.BindDN, opts.Password); err != nil {
		return fmt.Errorf("%w: bind failed: %v", plugin.ErrUnauthorized, err)
	}
	return nil
}

// discoverBaseDN reads the server's root DSE to pick a default naming context
// when the connection config leaves the base DN blank.
func (s *Session) discoverBaseDN() (string, error) {
	res, err := s.conn.Search(ldapv3.NewSearchRequest(
		"", ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, int(s.opts.Timeout.Seconds()), false,
		"(objectClass=*)", []string{"defaultNamingContext", "namingContexts"}, nil,
	))
	if err != nil || len(res.Entries) == 0 {
		return "", fmt.Errorf("%w: could not read root DSE; set an explicit base DN", plugin.ErrUnavailable)
	}
	entry := res.Entries[0]
	if def := entry.GetAttributeValue("defaultNamingContext"); def != "" {
		return def, nil
	}
	if contexts := entry.GetAttributeValues("namingContexts"); len(contexts) > 0 {
		return contexts[0], nil
	}
	return "", fmt.Errorf("%w: server exposes no naming context; set an explicit base DN", plugin.ErrInvalidInput)
}

func (s *Session) HealthCheck(context.Context) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	_, err := s.conn.Search(ldapv3.NewSearchRequest(
		"", ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, int(s.opts.Timeout.Seconds()), false,
		"(objectClass=*)", []string{"namingContexts"}, nil,
	))
	if err != nil {
		return fmt.Errorf("%w: LDAP health check: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	if s == nil || !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	return nil
}

func (s *Session) ensureOpen() error {
	if s == nil || s.closed.Load() {
		return fmt.Errorf("%w: LDAP session closed", plugin.ErrUnavailable)
	}
	return nil
}

func unwrap(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	type sessionGetter interface{ Session() plugin.Session }
	if h, ok := sess.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: LDAP session unavailable", plugin.ErrUnavailable)
}
