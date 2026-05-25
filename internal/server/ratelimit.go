package server

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter is a small per-key token-bucket limiter used to throttle
// unauthenticated endpoints (login) against online brute force. Idle keys are
// swept opportunistically so the map cannot grow without bound.
type rateLimiter struct {
	mu        sync.Mutex
	clients   map[string]*rateClient
	limit     rate.Limit
	burst     int
	ttl       time.Duration
	lastSweep time.Time
	now       func() time.Time
}

type rateClient struct {
	limiter *rate.Limiter
	seen    time.Time
}

// newRateLimiter builds a limiter allowing burst requests up to limit/sec per key.
func newRateLimiter(limit rate.Limit, burst int) *rateLimiter {
	return &rateLimiter{
		clients: map[string]*rateClient{},
		limit:   limit,
		burst:   burst,
		ttl:     15 * time.Minute,
		now:     time.Now,
	}
}

// allow reports whether the key may proceed, consuming one token if so.
func (rl *rateLimiter) allow(key string) bool {
	now := rl.now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if now.Sub(rl.lastSweep) > rl.ttl {
		cutoff := now.Add(-rl.ttl)
		for k, c := range rl.clients {
			if c.seen.Before(cutoff) {
				delete(rl.clients, k)
			}
		}
		rl.lastSweep = now
	}
	c, ok := rl.clients[key]
	if !ok {
		c = &rateClient{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.clients[key] = c
	}
	c.seen = now
	return c.limiter.Allow()
}

// loginRateLimit throttles login attempts per source IP.
func (s *Server) loginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.loginLimiter != nil && !s.loginLimiter.allow(clientIP(r)) {
			writeJSON(w, http.StatusTooManyRequests, errorEnvelope{Error: "too many attempts; try again later"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
