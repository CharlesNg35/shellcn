package app

import (
	"strings"

	"github.com/charlesng35/shellcn/internal/cache"
)

// RedisClientConfig converts the application cache configuration into the cache package representation.
func (c CacheConfig) RedisClientConfig() cache.RedisConfig {
	return cache.RedisConfig{
		Address:  strings.TrimSpace(c.Redis.Address),
		Username: strings.TrimSpace(c.Redis.Username),
		Password: c.Redis.Password,
		DB:       c.Redis.DB,
		TLS:      c.Redis.TLS,
		Timeout:  c.Redis.Timeout,
	}
}
