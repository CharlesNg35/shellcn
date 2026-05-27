package postgresql

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/charlesng35/shellcn/internal/plugin"
)

// Session brokers a PostgreSQL connection. A connection can browse every
// database in the cluster, so pools are opened lazily per database (keyed by
// name) and reused; the configured database is the default when none is named.
type Session struct {
	opts   options
	net    plugin.NetTransport
	baseDB string

	mu      sync.Mutex
	pools   map[string]*pgxpool.Pool
	running map[string]context.CancelFunc
}

func connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	s := &Session{
		opts:    opts,
		net:     cfg.Net,
		baseDB:  opts.Database,
		pools:   map[string]*pgxpool.Pool{},
		running: map[string]context.CancelFunc{},
	}
	if _, err := s.poolFor(ctx, opts.Database); err != nil {
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
	return nil, fmt.Errorf("%w: PostgreSQL session unavailable", plugin.ErrUnavailable)
}

// poolFor returns the pool for the named database, opening and caching it on
// first use. An empty name resolves to the connection's configured database.
func (s *Session) poolFor(ctx context.Context, database string) (*pgxpool.Pool, error) {
	database = strings.TrimSpace(database)
	if database == "" {
		database = s.baseDB
	}
	s.mu.Lock()
	if pool := s.pools[database]; pool != nil {
		s.mu.Unlock()
		return pool, nil
	}
	s.mu.Unlock()

	pc, err := poolConfig(s.opts, s.net)
	if err != nil {
		return nil, err
	}
	pc.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("%w: open PostgreSQL pool: %v", plugin.ErrUnavailable, err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%w: PostgreSQL ping %q: %v", plugin.ErrUnavailable, database, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.pools[database]; existing != nil {
		pool.Close()
		return existing, nil
	}
	s.pools[database] = pool
	return pool, nil
}

// closePool closes and forgets the cached pool for a database (e.g. before
// dropping it, since an open connection blocks DROP DATABASE). The base pool is
// never closed this way.
func (s *Session) closePool(database string) {
	database = strings.TrimSpace(database)
	if database == "" || database == s.baseDB {
		return
	}
	s.mu.Lock()
	pool := s.pools[database]
	delete(s.pools, database)
	s.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
}

func (s *Session) HealthCheck(ctx context.Context) error {
	pool, err := s.poolFor(ctx, s.baseDB)
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("%w: PostgreSQL ping: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	for id, cancel := range s.running {
		cancel()
		delete(s.running, id)
	}
	pools := s.pools
	s.pools = map[string]*pgxpool.Pool{}
	s.mu.Unlock()
	for _, pool := range pools {
		pool.Close()
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) track(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	s.running[id] = cancel
	s.mu.Unlock()
}

func (s *Session) untrack(id string) {
	s.mu.Lock()
	delete(s.running, id)
	s.mu.Unlock()
}

func (s *Session) cancelAll() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancelled := false
	for id, cancel := range s.running {
		cancel()
		delete(s.running, id)
		cancelled = true
	}
	return cancelled
}
