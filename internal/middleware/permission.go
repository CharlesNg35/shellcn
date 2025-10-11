package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/metrics"
	"github.com/charlesng35/shellcn/pkg/response"
)

// RequirePermission checks that the authenticated user has the provided permission ID.
func RequirePermission(checker *permissions.Checker, permissionID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(CtxUserIDKey)
		if !ok {
			response.Error(c, errors.ErrUnauthorized)
			c.Abort()
			return
		}
		userID, _ := v.(string)
		allowed, err := checker.Check(c.Request.Context(), userID, permissionID)
		if err != nil {
			// Internal error while checking permissions
			metrics.PermissionChecks.WithLabelValues(permissionID, "error").Inc()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": errors.ErrInternalServer.Code, "message": "permission check failed"}})
			return
		}
		if !allowed {
			metrics.PermissionChecks.WithLabelValues(permissionID, "denied").Inc()
			response.Error(c, errors.ErrForbidden)
			c.Abort()
			return
		}
		metrics.PermissionChecks.WithLabelValues(permissionID, "allowed").Inc()
		c.Next()
	}
}
