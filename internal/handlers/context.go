package handlers

import (
	"context"

	"github.com/gin-gonic/gin"
)

// requestContext safely returns the request context with a background fallback for tests.
func requestContext(c *gin.Context) context.Context {
	if c == nil {
		return context.Background()
	}
	if req := c.Request; req != nil {
		return req.Context()
	}
	return context.Background()
}
