package session

import (
	"context"
	"sync"

	"github.com/charlesng/shellcn/internal/plugin"
)

// Handle is a borrowed reference to a live session. It opens tracked channels
// and keeps the session marked as recently used.
type Handle struct {
	m *Manager
	e *entry
}

// Session returns the underlying plugin session.
func (h *Handle) Session() plugin.Session {
	h.e.mu.Lock()
	defer h.e.mu.Unlock()
	return h.e.sess
}

// OpenChannel opens a tracked upstream stream, enforcing the per-session channel
// cap. The returned channel decrements the counter exactly once on Close.
func (h *Handle) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	e := h.e
	e.mu.Lock()
	if e.closed || e.sess == nil {
		e.mu.Unlock()
		return nil, ErrSessionClosed
	}
	if e.channels >= h.m.opts.MaxChannelsPerSession {
		e.mu.Unlock()
		return nil, ErrChannelLimit
	}
	e.channels++
	e.lastUsed = h.m.now()
	sess := e.sess
	e.mu.Unlock()

	ch, err := sess.OpenChannel(ctx, req)
	if err != nil {
		e.mu.Lock()
		e.channels--
		e.mu.Unlock()
		return nil, err
	}
	return &trackedChannel{Channel: ch, release: func() {
		e.mu.Lock()
		if e.channels > 0 {
			e.channels--
		}
		e.lastUsed = h.m.now()
		e.mu.Unlock()
	}}, nil
}

// trackedChannel decrements the session's channel counter once, on Close.
type trackedChannel struct {
	plugin.Channel
	once    sync.Once
	release func()
}

func (c *trackedChannel) Close() error {
	err := c.Channel.Close()
	c.once.Do(c.release)
	return err
}
