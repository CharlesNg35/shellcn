package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit returns a middleware that limits requests per (clientIP,path) within a fixed window.
// This is an in-memory limiter suitable for single-instance deployments and tests.
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	type counter struct {
		count     int
		windowEnd time.Time
	}

	var (
		mu   sync.Mutex
		data = make(map[string]*counter)
	)

	tick := time.NewTicker(window)
	// Periodically cleanup old counters to avoid unbounded growth
	go func() {
		for range tick.C {
			now := time.Now()
			mu.Lock()
			for k, v := range data {
				if now.After(v.windowEnd) {
					delete(data, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		if maxRequests <= 0 || window <= 0 {
			c.Next()
			return
		}

		key := c.ClientIP() + "|" + c.FullPath()
		now := time.Now()

		mu.Lock()
		ct, ok := data[key]
		if !ok || now.After(ct.windowEnd) {
			ct = &counter{count: 0, windowEnd: now.Add(window)}
			data[key] = ct
		}
		ct.count++
		remaining := maxRequests - ct.count
		resetIn := time.Until(ct.windowEnd)
		mu.Unlock()

		c.Header("X-RateLimit-Limit", itoa(maxRequests))
		c.Header("X-RateLimit-Remaining", itoa(max(0, remaining)))
		c.Header("X-RateLimit-Reset", itoa(int(resetIn.Seconds())))

		if ct.count > maxRequests {
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
