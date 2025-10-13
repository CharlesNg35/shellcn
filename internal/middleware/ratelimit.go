package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit returns a middleware that limits requests per (clientIP,path) within a fixed window.
// It uses the provided store for distributed coordination and falls back to an in-memory store when unavailable.
func RateLimit(store RateStore, maxRequests int, window time.Duration) gin.HandlerFunc {
	fallback := NewMemoryRateStore()
	if store == nil {
		store = fallback
	}
	var secondary RateStore
	if store != fallback {
		secondary = fallback
	}

	return func(c *gin.Context) {
		if maxRequests <= 0 || window <= 0 {
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" && c.Request != nil {
			path = c.Request.URL.Path
		}
		key := c.ClientIP() + "|" + path
		ctx := c.Request.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		count, ttl, err := store.Increment(ctx, key, window)
		if err != nil && secondary != nil {
			count, ttl, _ = secondary.Increment(ctx, key, window)
		}

		if ttl <= 0 {
			ttl = window
		}

		remaining := maxRequests - count
		resetIn := int(ttl.Round(time.Second) / time.Second)
		if resetIn < 0 {
			resetIn = 0
		}

		c.Header("X-RateLimit-Limit", itoa(maxRequests))
		c.Header("X-RateLimit-Remaining", itoa(max(0, remaining)))
		c.Header("X-RateLimit-Reset", itoa(resetIn))

		if count > maxRequests {
			// 429 Too Many Requests
			c.AbortWithStatus(429)
			return
		}

		c.Next()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func itoa(i int) string { return fmtInt(i) }

// small, allocation-free int to string
func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
