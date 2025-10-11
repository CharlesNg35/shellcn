package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/charlesng35/shellcn/pkg/logger"
	"github.com/charlesng35/shellcn/pkg/response"
)

// Recovery converts panics into a 500 response and logs the error.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.WithModule("http").Error("panic",
					zap.String("path", c.Request.URL.Path),
					zap.Any("error", r),
				)
				// Avoid leaking internals to clients
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "INTERNAL_SERVER_ERROR",
						"message": "Internal server error",
					},
				})
			}
		}()
		c.Next()
	}
}

// NotFoundHandler returns a JSON 404 response for unknown routes.
func NotFoundHandler(c *gin.Context) {
	response.Success(c, http.StatusNotFound, gin.H{"error": fmt.Sprintf("route %s not found", c.Request.URL.Path)})
}
