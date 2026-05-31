package server

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxSession
)

func userFrom(ctx context.Context) (models.User, bool) {
	u, ok := ctx.Value(ctxUser).(models.User)
	return u, ok
}

func sessionFrom(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(ctxSession).(auth.Session)
	return s, ok
}

// authenticate resolves the session cookie to a live user and returns a context
// carrying the user + session. It writes the error response and reports false on
// any failure. It does not enforce CSRF — that is the caller's choice.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (context.Context, bool) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		writeAuthRequired(w, s.deps.Logger, plugin.ErrUnauthorized)
		return nil, false
	}
	sess, ok := s.deps.SessionMgr.Get(cookie.Value)
	if !ok {
		writeAuthRequired(w, s.deps.Logger, plugin.ErrUnauthorized)
		return nil, false
	}
	user, err := s.deps.Store.Users.GetByID(r.Context(), sess.UserID)
	if err != nil {
		writeAuthRequired(w, s.deps.Logger, plugin.ErrUnauthorized)
		return nil, false
	}
	if user.Disabled || sess.SessionVersion != user.SessionVersion {
		s.deps.SessionMgr.Destroy(sess.ID)
		auth.ClearSessionCookie(w)
		writeAuthRequired(w, s.deps.Logger, plugin.ErrUnauthorized)
		return nil, false
	}
	ctx := context.WithValue(r.Context(), ctxUser, user)
	ctx = context.WithValue(ctx, ctxSession, sess)
	return ctx, true
}

// requireAuth authenticates the session and enforces CSRF on state-changing
// methods. It guards every browser API route that carries our CSRF token.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := s.authenticate(w, r)
		if !ok {
			return
		}
		if isStateChanging(r.Method) {
			if sess, _ := sessionFrom(ctx); !sess.ValidateCSRF(r) {
				writeError(w, s.deps.Logger, plugin.ErrForbidden)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireSession authenticates the session without the CSRF-token check. It backs
// the connection web proxy, where a proxied third-party app cannot carry our
// token; cross-site forgery is still blocked by the SameSite=Lax session cookie,
// and the route itself authorizes the connection.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, ok := s.authenticate(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin gates a route group to platform admins (used for user/role and
// invitation management). It runs inside the authenticated group.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := userFrom(r.Context())
		if !user.HasRole(models.RoleAdmin) {
			writeError(w, s.deps.Logger, plugin.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isTLS(r *http.Request) bool {
	return r.TLS != nil || forwardedProto(r) == "https"
}

func requestHost(r *http.Request) string {
	if host := forwardedValue(r.Header.Get("Forwarded"), "host"); host != "" {
		return host
	}
	if host := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); host != "" {
		return host
	}
	return r.Host
}

func forwardedProto(r *http.Request) string {
	if proto := forwardedValue(r.Header.Get("Forwarded"), "proto"); proto != "" {
		return strings.ToLower(proto)
	}
	return strings.ToLower(firstHeaderValue(r.Header.Get("X-Forwarded-Proto")))
}

func firstHeaderValue(value string) string {
	if value == "" {
		return ""
	}
	value = strings.Split(value, ",")[0]
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func forwardedValue(header, key string) string {
	if header == "" {
		return ""
	}
	part := strings.Split(header, ",")[0]
	for _, pair := range strings.Split(part, ";") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok || strings.ToLower(strings.TrimSpace(name)) != key {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}

// withRemoteAddr stashes the direct peer address on the request context so every
// audit event recorded during the request carries the caller's source IP. The
// non-spoofable RemoteAddr is used (not X-Forwarded-For) so the audit trail
// cannot be forged by a client header.
func (s *Server) withRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := audit.WithRemoteAddr(r.Context(), clientIP(r))
		ctx = audit.WithSource(ctx, audit.SourceHTTP, "")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
