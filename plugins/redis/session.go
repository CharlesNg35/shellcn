package redis

import (
	"context"
	"fmt"
	"sync/atomic"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/charlesng35/shellcn/internal/plugin"
)

type Session struct {
	client *redisclient.Client
	opts   options
	closed atomic.Bool
}

func connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	clientOpts, err := clientOptions(opts, cfg.Net)
	if err != nil {
		return nil, err
	}
	client := redisclient.NewClient(clientOpts)
	s := &Session{client: client, opts: opts}
	if err := s.HealthCheck(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func unwrap(sess plugin.Session) (*Session, error) {
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
	return nil, fmt.Errorf("%w: Redis session unavailable", plugin.ErrUnavailable)
}

func (s *Session) HealthCheck(ctx context.Context) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: Redis ping: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	s.closed.Store(true)
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Session) ensureOpen() error {
	if s == nil || s.closed.Load() {
		return fmt.Errorf("%w: Redis session closed", plugin.ErrUnavailable)
	}
	return nil
}
