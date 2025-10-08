package cache

import (
	"context"
	"time"
)

// Store represents a shared cache interface used across the application.
type Store interface {
	IncrementWithTTL(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, keys ...string) error
}
