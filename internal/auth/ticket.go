package auth

import (
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// DefaultTicketTTL is the short lifetime of a WS ticket.
const DefaultTicketTTL = 30 * time.Second

// ErrTicketInvalid is returned for any ticket failure (unknown, expired, used,
// or scope mismatch) — deliberately undifferentiated so it leaks nothing.
var ErrTicketInvalid = errors.New("auth: invalid ticket")

// TicketScope binds a ticket to exactly one resource + acting user. Binding the
// params means a ticket minted for pod-A can't be replayed against pod-B.
type TicketScope struct {
	ConnectionID string
	RouteID      string
	Params       map[string]string
	UserID       string
}

type ticket struct {
	scope     TicketScope
	expiresAt time.Time
}

// TicketStore mints and redeems single-use, short-lived WS tickets. Browsers
// can't set Authorization on a WS upgrade, so tickets are mandatory. The store
// is in-memory and not shared across instances.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticket
	ttl     time.Duration
}

// NewTicketStore returns a store with the given TTL (0 = default).
func NewTicketStore(ttl time.Duration) *TicketStore {
	if ttl <= 0 {
		ttl = DefaultTicketTTL
	}
	return &TicketStore{tickets: make(map[string]ticket), ttl: ttl}
}

// Mint issues a token bound to scope, expiring after the store's TTL.
func (s *TicketStore) Mint(scope TicketScope) (token string, expiresAt time.Time) {
	token = randomToken()
	expiresAt = time.Now().Add(s.ttl)
	s.mu.Lock()
	s.tickets[token] = ticket{scope: cloneScope(scope), expiresAt: expiresAt}
	s.mu.Unlock()
	return token, expiresAt
}

// Redeem validates a token against the request's scope and consumes it
// (single-use). Any mismatch, expiry, or reuse returns ErrTicketInvalid.
func (s *TicketStore) Redeem(token string, want TicketScope) error {
	s.mu.Lock()
	t, ok := s.tickets[token]
	if ok {
		delete(s.tickets, token) // single-use: gone whether or not it validates
	}
	s.mu.Unlock()

	if !ok || time.Now().After(t.expiresAt) {
		return ErrTicketInvalid
	}
	if t.scope.ConnectionID != want.ConnectionID ||
		t.scope.RouteID != want.RouteID ||
		t.scope.UserID != want.UserID ||
		!sameParams(t.scope.Params, want.Params) {
		return ErrTicketInvalid
	}
	return nil
}

func cloneScope(s TicketScope) TicketScope {
	out := s
	if s.Params != nil {
		out.Params = make(map[string]string, len(s.Params))
		for k, v := range s.Params {
			out.Params[k] = v
		}
	}
	return out
}

func sameParams(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// CheckWSOrigin reports whether the request's Origin header is same-site with
// the Host (or matches an explicit allowlist). A missing Origin is rejected for
// cross-origin safety on the WS upgrade path.
func CheckWSOrigin(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, a := range allowed {
		if u.Host == a {
			return true
		}
	}
	return false
}
