package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/monitoring"
)

// Metrics records request latency metrics for each HTTP request.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		status := strconv.Itoa(c.Writer.Status())
		monitoring.ObserveAPILatency(c.Request.Method, path, status, duration)
	}
}
