package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/charlesng35/shellcn/pkg/crypto"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/logger"
	"github.com/charlesng35/shellcn/pkg/response"
)

const (
	// CSRFCookieName is the cookie used to transport the CSRF token to clients.
	CSRFCookieName = "shellcn_csrf"
	// CSRFHeaderName is the header clients must present for unsafe HTTP methods.
	CSRFHeaderName = "X-CSRF-Token"

	csrfTokenLength  = 48
	csrfCookieMaxAge = 12 * 60 * 60 // 12 hours
	csrfLoggerModule = "csrf"
)

var unsafeMethods = map[string]struct{}{
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

// CSRF implements the double-submit-cookie pattern to protect cookie-authenticated
// clients against CSRF attacks. Safe methods receive a token via cookie and header,
// while mutating requests must echo the token using the X-CSRF-Token header.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == http.MethodOptions {
			c.Next()
			return
		}

		token, issued, err := ensureCSRFCookie(c)
		if err != nil {
			response.Error(c, errors.ErrInternalServer)
			c.Abort()
			return
		}

		if isUnsafeMethod(method) {
			headerToken := strings.TrimSpace(c.GetHeader(CSRFHeaderName))
			if headerToken == "" || !constantTimeEqual(token, headerToken) {
				logger.WithModule(csrfLoggerModule).Warn("csrf validation failed",
					// Avoid logging token contents
					zap.String("method", method),
					zap.String("path", c.FullPath()),
					zap.Bool("cookie_issued", issued),
				)
				response.Error(c, errors.ErrCSRFInvalid)
				c.Abort()
				return
			}
		} else {
			c.Header(CSRFHeaderName, token)
		}

		c.Next()
	}
}

func ensureCSRFCookie(c *gin.Context) (token string, issued bool, err error) {
	if existing, err := c.Cookie(CSRFCookieName); err == nil && len(existing) > 0 {
		setCSRFCookie(c, existing)
		return existing, false, nil
	}

	token, err = crypto.GenerateToken(csrfTokenLength)
	if err != nil {
		return "", false, err
	}
	setCSRFCookie(c, token)
	return token, true, nil
}

func setCSRFCookie(c *gin.Context, token string) {
	secure := isSecureRequest(c.Request)
	c.SetSameSite(http.SameSiteStrictMode)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Secure:   secure,
		HttpOnly: false,
		MaxAge:   csrfCookieMaxAge,
		SameSite: http.SameSiteStrictMode,
	})
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	return strings.EqualFold(scheme, "https")
}

func isUnsafeMethod(method string) bool {
	_, ok := unsafeMethods[method]
	return ok
}

func constantTimeEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
