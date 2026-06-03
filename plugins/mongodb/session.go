package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Session struct {
	client *mongo.Client
	opts   optionsData
}

func connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	opts, err := parseOptions(cfg)
	if err != nil {
		return nil, err
	}
	co, err := clientOptions(opts, cfg.Net)
	if err != nil {
		return nil, err
	}
	client, err := mongo.Connect(co)
	if err != nil {
		return nil, fmt.Errorf("%w: open MongoDB client: %v", plugin.ErrUnavailable, err)
	}
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
	return nil, fmt.Errorf("%w: MongoDB session unavailable", plugin.ErrUnavailable)
}

func (s *Session) HealthCheck(ctx context.Context) error {
	ctx, cancel := commandContext(ctx, s)
	defer cancel()
	if err := s.client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("%w: MongoDB ping: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.Timeout)
	defer cancel()
	return s.client.Disconnect(ctx)
}
