package servermonitor

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Session struct {
	backend  hostmonitor.Backend
	interval time.Duration
}

func Connect(ctx context.Context, cfg plugin.ConnectConfig) (plugin.Session, error) {
	var backend hostmonitor.Backend
	processLimit, ok := cfg.Int("process_limit")
	if !ok || processLimit <= 0 {
		processLimit = 1000
	}
	connectionLimit, ok := cfg.Int("connection_limit")
	if !ok || connectionLimit <= 0 {
		connectionLimit = 1000
	}
	if cfg.Transport == plugin.TransportAgent {
		baseURL, rt, ok := cfg.Net.HTTP()
		if !ok {
			return nil, fmt.Errorf("%w: server monitor agent must expose host_monitor HTTP transport", plugin.ErrUnavailable)
		}
		backend = hostmonitor.NewRemote(baseURL, &http.Client{Transport: rt})
	} else {
		backend = hostmonitor.NewLocal(hostmonitor.Options{ProcessLimit: processLimit, ConnectionLimit: connectionLimit})
	}
	s := &Session{backend: backend, interval: intervalFromConfig(cfg)}
	if err := s.HealthCheck(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) HealthCheck(ctx context.Context) error {
	_, err := s.backend.Overview(ctx)
	if err != nil {
		return fmt.Errorf("%w: server monitor: %v", plugin.ErrUnavailable, err)
	}
	return nil
}

func (s *Session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *Session) Close() error { return nil }

func sess(rc *plugin.RequestContext) (*Session, error) {
	s, ok := rc.Session.(*Session)
	if ok {
		return s, nil
	}
	type sessionGetter interface{ Session() plugin.Session }
	if h, ok := rc.Session.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: server monitor session unavailable", plugin.ErrUnavailable)
}
