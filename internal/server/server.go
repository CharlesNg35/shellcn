// Package server is the HTTP/WS adapter: it mounts plugin routes behind the full
// middleware chain (authn → authz → session → validate → audit → handler →
// normalize), exposes the projection + catalog APIs, and serves the embedded UI.
package server

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
	"github.com/charlesng/shellcn/internal/service"
	"github.com/charlesng/shellcn/internal/session"
	"github.com/charlesng/shellcn/internal/store"
	"github.com/charlesng/shellcn/internal/telemetry"
)

// Deps are the server's injected dependencies (wired once in cmd/server).
type Deps struct {
	Plugins     *plugin.Registry
	Store       *store.Store
	Sessions    *session.Manager
	Auth        auth.Authenticator
	SessionMgr  *auth.SessionManager
	Tickets     *auth.TicketStore
	Policy      *policy.Enforcer
	Connector   *service.Connector
	Credentials *service.CredentialService
	Audit       audit.Sink
	Metrics     *telemetry.Metrics
	Health      *telemetry.Health
	Logger      *slog.Logger

	// StaticFS is the embedded web/dist (nil in dev mode, where Vite serves the UI).
	StaticFS fs.FS
	Dev      bool
	// AllowedOrigins are extra WS origins beyond same-site (usually empty).
	AllowedOrigins []string
}

// Server wires the dependencies into a chi router.
type Server struct {
	deps   Deps
	router chi.Router
}

// New builds the server and its routes.
func New(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Audit == nil {
		d.Audit = audit.Noop{}
	}
	s := &Server{deps: d}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(telemetry.RequestIDMiddleware)

	// Observability endpoints (unauthenticated, like any /metrics + /healthz).
	if s.deps.Health != nil {
		r.Get("/healthz", s.deps.Health.Handler())
	} else {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	}
	if s.deps.Metrics != nil {
		r.Handle("/metrics", s.deps.Metrics.Handler())
	}

	r.Route("/api", func(api chi.Router) {
		// Auth (login is public; the rest require a session).
		api.Post("/auth/login", s.handleLogin)
		api.Group(func(pr chi.Router) {
			pr.Use(s.requireAuth)
			pr.Post("/auth/logout", s.handleLogout)
			pr.Get("/auth/me", s.handleMe)

			pr.Get("/plugins", s.handleListPlugins)
			pr.Get("/plugins/{name}", s.handleGetPlugin)
			pr.Get("/connections", s.handleListConnections)
			pr.Get("/credentials", s.handleListCredentials)

			pr.Post("/connections/{id}/tickets", s.handleMintTicket)
			pr.HandleFunc("/connections/{id}/x/{routeID}", s.handleRoute)
		})
	})

	// SPA: serve the embedded UI with history fallback (API already matched above).
	if !s.deps.Dev && s.deps.StaticFS != nil {
		r.NotFound(s.spaHandler())
	}
	return r
}
