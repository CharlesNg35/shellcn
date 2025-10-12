package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/auditctx"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

const (
	CtxClaimsKey    = "authClaims"
	CtxUserIDKey    = "userID"
	CtxSessionIDKey = "sessionID"
)

// Auth enforces JWT authentication using the supplied JWT service.
func Auth(jwt *iauth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		if len(authz) < 8 || !strings.EqualFold(authz[:7], "Bearer ") {
			response.Error(c, errors.ErrUnauthorized)
			c.Abort()
			return
		}

		token := strings.TrimSpace(authz[7:])
		claims, err := jwt.ValidateAccessToken(token)
		if err != nil {
			// Normalise all validation failures to 401
			c.Header("WWW-Authenticate", "Bearer")
			response.Error(c, errors.ErrUnauthorized)
			c.Abort()
			return
		}

		// Propagate identity into request context
		c.Set(CtxClaimsKey, claims)
		c.Set(CtxUserIDKey, claims.UserID)
		if claims.SessionID != "" {
			c.Set(CtxSessionIDKey, claims.SessionID)
		}

		actor := auditctx.Actor{
			UserID:    claims.UserID,
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}
		if claims.Metadata != nil {
			if username, ok := claims.Metadata["username"].(string); ok {
				actor.Username = strings.TrimSpace(username)
			}
		}
		if actor.Username == "" {
			actor.Username = claims.UserID
		}

		ctx := auditctx.WithActor(c.Request.Context(), actor)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
