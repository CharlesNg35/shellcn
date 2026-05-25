package server

import (
	"context"
	"net/http"

	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
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

// requireAuth resolves the session cookie to a live user, enforces CSRF on
// state-changing methods, and attaches the user + session to the context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil {
			writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
			return
		}
		sess, ok := s.deps.SessionMgr.Get(cookie.Value)
		if !ok {
			writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
			return
		}
		if isStateChanging(r.Method) && !sess.ValidateCSRF(r) {
			writeError(w, s.deps.Logger, plugin.ErrForbidden)
			return
		}
		user, err := s.deps.Store.Users.GetByID(r.Context(), sess.UserID)
		if err != nil {
			writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
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
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
