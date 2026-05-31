package modelreg

import (
	"sync"
	"time"
)

// ttlCache is a concurrency-safe single-value cache with a TTL plus a stale
// fallback, so a failed refresh can still serve the last good value.
type ttlCache[T any] struct {
	mu        sync.Mutex
	ttl       time.Duration
	now       func() time.Time
	value     T
	hasValue  bool
	fetchedAt time.Time
}

func newTTLCache[T any](ttl time.Duration, now func() time.Time) *ttlCache[T] {
	if now == nil {
		now = time.Now
	}
	return &ttlCache[T]{ttl: ttl, now: now}
}

// get returns the cached value if it is still fresh.
func (c *ttlCache[T]) get() (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hasValue && c.now().Sub(c.fetchedAt) < c.ttl {
		return c.value, true
	}
	var zero T
	return zero, false
}

// getStale returns the last value regardless of age.
func (c *ttlCache[T]) getStale() (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, c.hasValue
}

func (c *ttlCache[T]) set(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	c.hasValue = true
	c.fetchedAt = c.now()
}

// getOrFetch returns the fresh value, fetching (and caching) when expired. On a
// fetch error it returns the stale value if one exists.
func (c *ttlCache[T]) getOrFetch(fetch func() (T, error)) (T, error) {
	if v, ok := c.get(); ok {
		return v, nil
	}
	v, err := fetch()
	if err != nil {
		if stale, ok := c.getStale(); ok {
			return stale, nil
		}
		var zero T
		return zero, err
	}
	c.set(v)
	return v, nil
}
