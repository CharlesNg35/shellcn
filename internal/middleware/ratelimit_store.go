package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/charlesng35/shellcn/internal/cache"
)

// RateStore coordinates rate limiting counters for a specific key.
type RateStore interface {
	Increment(ctx context.Context, key string, window time.Duration) (count int, ttl time.Duration, err error)
}

// memoryRateStore provides process-local rate limiting. It is concurrency-safe.
type memoryRateStore struct {
	mu    sync.Mutex
	data  map[string]*memoryCounter
	tick  *time.Ticker
	clock func() time.Time
}

type memoryCounter struct {
	count     int
	windowEnd time.Time
}

// NewMemoryRateStore constructs an in-memory rate store.
func NewMemoryRateStore() RateStore {
	store := &memoryRateStore{
		data:  make(map[string]*memoryCounter),
		tick:  time.NewTicker(time.Minute),
		clock: time.Now,
	}

	go store.cleanupLoop()
	return store
}

func (s *memoryRateStore) cleanupLoop() {
	for range s.tick.C {
		now := s.clock()
		s.mu.Lock()
		for key, counter := range s.data {
			if now.After(counter.windowEnd) {
				delete(s.data, key)
			}
		}
		s.mu.Unlock()
	}
}

func (s *memoryRateStore) Increment(_ context.Context, key string, window time.Duration) (int, time.Duration, error) {
	if window <= 0 {
		window = time.Minute
	}

	now := s.clock()

	s.mu.Lock()
	defer s.mu.Unlock()

	counter, ok := s.data[key]
	if !ok || now.After(counter.windowEnd) {
		counter = &memoryCounter{
			count:     0,
			windowEnd: now.Add(window),
		}
		s.data[key] = counter
	}

	counter.count++

	return counter.count, time.Until(counter.windowEnd), nil
}

// redisRateStore implements RateStore backed by Redis.
type storeRateStore struct {
	store cache.Store
}

// NewRedisRateStore wraps a Redis-backed cache store in a RateStore implementation.
func NewRedisRateStore(store cache.Store) RateStore {
	return newStoreRateStore(store)
}

// NewDatabaseRateStore builds a RateStore based on the SQL database cache.
func NewDatabaseRateStore(store cache.Store) RateStore {
	return newStoreRateStore(store)
}

func newStoreRateStore(store cache.Store) RateStore {
	if store == nil {
		return nil
	}
	return &storeRateStore{store: store}
}

func (s *storeRateStore) Increment(ctx context.Context, key string, window time.Duration) (int, time.Duration, error) {
	if window <= 0 {
		window = time.Minute
	}
	count, ttl, err := s.store.IncrementWithTTL(ctx, key, window)
	return int(count), ttl, err
}
