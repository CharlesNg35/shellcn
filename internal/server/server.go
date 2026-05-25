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
	"github.com/charlesng/shellcn/internal/transport"
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
	Connections *service.ConnectionService
	Credentials *service.CredentialService
	Enrollments *service.EnrollmentService
	Users       *service.UserService
	Invitations *service.InvitationService
	Tunnels     *transport.Registry
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

		// The agent connect endpoint authenticates with its enrollment token in
		// the handshake (it is not a browser session), so it sits outside the
		// session-guarded group.
		if s.deps.Enrollments != nil && s.deps.Tunnels != nil {
			api.Get("/agent/connect", s.handleAgentConnect)
		}

		// Invitation acceptance is public (the invitee has no session yet).
		if s.deps.Invitations != nil {
			api.Get("/invitations/{token}", s.handleInvitationLookup)
			api.Post("/invitations/{token}/accept", s.handleAcceptInvitation)
		}

		api.Group(func(pr chi.Router) {
			pr.Use(s.requireAuth)
			pr.Post("/auth/logout", s.handleLogout)
			pr.Get("/auth/me", s.handleMe)

			pr.Get("/plugins", s.handleListPlugins)
			pr.Get("/plugins/{name}", s.handleGetPlugin)

			pr.Get("/connections", s.handleListConnections)
			pr.Get("/credentials", s.handleListCredentials)

			if s.deps.Connections != nil {
				pr.Post("/connections", s.handleCreateConnection)
				pr.Get("/connections/{id}", s.handleConnectionDetail)
				pr.Put("/connections/{id}", s.handleUpdateConnection)
				pr.Delete("/connections/{id}", s.handleDeleteConnection)
			}
			if s.deps.Credentials != nil {
				pr.Post("/credentials", s.handleCreateCredential)
				pr.Put("/credentials/{id}", s.handleUpdateCredential)
				pr.Delete("/credentials/{id}", s.handleDeleteCredential)
			}

			pr.Get("/users", s.handleListUsers)
			if s.deps.Connections != nil {
				pr.Get("/connections/{id}/grants", s.handleListConnectionGrants)
				pr.Post("/connections/{id}/grants", s.handleCreateConnectionGrant)
				pr.Delete("/connections/{id}/grants/{grantId}", s.handleDeleteConnectionGrant)
			}
			if s.deps.Credentials != nil {
				pr.Get("/credentials/{id}/grants", s.handleListCredentialGrants)
				pr.Post("/credentials/{id}/grants", s.handleCreateCredentialGrant)
				pr.Delete("/credentials/{id}/grants/{grantId}", s.handleDeleteCredentialGrant)
			}

			pr.Post("/connections/{id}/tickets", s.handleMintTicket)
			if s.deps.Enrollments != nil {
				pr.Post("/connections/{id}/agent/enrollments", s.handleCreateEnrollment)
				pr.Get("/connections/{id}/agent/state", s.handleAgentState)
			}

			// Admin-only: user/role + invitation management.
			if s.deps.Users != nil {
				pr.Group(func(ar chi.Router) {
					ar.Use(s.requireAdmin)
					ar.Get("/admin/users", s.handleAdminListUsers)
					ar.Post("/admin/users", s.handleAdminCreateUser)
					ar.Put("/admin/users/{id}", s.handleAdminUpdateUser)
					ar.Delete("/admin/users/{id}", s.handleAdminDeleteUser)
					if s.deps.Invitations != nil {
						ar.Get("/admin/email", s.handleAdminEmailStatus)
						ar.Get("/admin/invitations", s.handleAdminListInvitations)
						ar.Post("/admin/invitations", s.handleAdminCreateInvitation)
						ar.Delete("/admin/invitations/{id}", s.handleAdminRevokeInvitation)
					}
				})
			}
			pr.HandleFunc("/connections/{id}/x/{routeID}", s.handleRoute)
		})
	})

	// SPA: serve the embedded UI with history fallback (API already matched above).
	if !s.deps.Dev && s.deps.StaticFS != nil {
		r.NotFound(s.spaHandler())
	}
	return r
}
