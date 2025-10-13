package middleware

import "github.com/gin-gonic/gin"

const (
	// DefaultContentSecurityPolicy restricts resources to same origin, with exceptions for fonts and styles.
	DefaultContentSecurityPolicy = "default-src 'self'; font-src 'self' https://fonts.gstatic.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; connect-src 'self' ws: wss:"
)

// SecurityHeaders applies common HTTP response headers that harden the API against
// clickjacking, MIME sniffing, basic XSS, and enforces HTTPS transport.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", DefaultContentSecurityPolicy)
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}
