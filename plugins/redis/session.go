package redis

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Session struct {
	client *redisclient.Client
	opts   options
	closed atomic.Bool

	mu        sync.Mutex
	dbClients map[int]*redisclient.Client
}

// scopedClient returns a client bound to db, reusing the session's primary
// client for the default database and lazily caching one sub-client per other
// database so repeated requests don't churn connection pools.
func (s *Session) scopedClient(db int) *redisclient.Client {
	if db == s.opts.Database {
		return s.client
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.dbClients[db]; ok {
		return c
	}
	opts := *s.client.Options()
	opts.DB = db
	c := redisclient.NewClient(&opts)
	if s.dbClients == nil {
		s.dbClients = make(map[int]*redisclient.Client)
	}
	s.dbClients[db] = c
	return c
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
	s.mu.Lock()
	for db, c := range s.dbClients {
		_ = c.Close()
		delete(s.dbClients, db)
	}
	s.mu.Unlock()
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
