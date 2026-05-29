// Package server is the HTTP/WS adapter: it mounts plugin routes behind the full
// middleware chain (authn → authz → session → validate → audit → handler →
// normalize), exposes the projection + catalog APIs, and serves the embedded UI.
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/telemetry"
	"github.com/charlesng35/shellcn/internal/transport"
)

// Deps are the server's injected dependencies (wired once in cmd/server).
type Deps struct {
	Plugins    *plugin.Registry
	Store      *store.Store
	Sessions   *session.Manager
	Auth       auth.Authenticator
	SessionMgr *auth.SessionManager
	Tickets    *auth.TicketStore
	// ArtifactTickets guards public install-artifact fetches. It has a longer TTL
	// than Tickets (a human copies a URL and runs it) and never expires a WS.
	ArtifactTickets   *auth.TicketStore
	Policy            *policy.Enforcer
	Connector         *service.Connector
	Connections       *service.ConnectionService
	Credentials       *service.CredentialService
	Enrollments       *service.EnrollmentService
	Users             *service.UserService
	Invitations       *service.InvitationService
	Tunnels           *transport.Registry
	Recordings        *service.RecordingService
	Recording         *recording.Engine
	RecordingMaxChunk int64
	Audit             audit.Sink
	Metrics           *telemetry.Metrics
	Health            *telemetry.Health
	Logger            *slog.Logger

	// StaticFS is the embedded web/dist (nil in dev mode, where Vite serves the UI).
	StaticFS fs.FS
	Dev      bool
	// AllowedOrigins are extra WS origins beyond same-site (usually empty).
	AllowedOrigins []string
}

// Server wires the dependencies into a chi router.
type Server struct {
	deps         Deps
	router       chi.Router
	loginLimiter *rateLimiter
}

// New builds the server and its routes.
func New(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Audit == nil {
		d.Audit = audit.Noop{}
	}
	// ~5 login attempts/min per IP with a small burst — generous for humans,
	// punishing for online password guessing.
	s := &Server{deps: d, loginLimiter: newRateLimiter(rate.Every(12*time.Second), 5)}
	s.router = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(telemetry.RequestIDMiddleware)
	r.Use(s.withRemoteAddr)

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
		// Auth (login is public; the rest require a session). Rate-limited per IP
		// to blunt online brute force.
		api.With(s.loginRateLimit).Post("/auth/login", s.handleLogin)

		// The agent connect endpoint authenticates with its enrollment token in
		// the handshake (it is not a browser session), so it sits outside the
		// session-guarded group.
		if s.deps.Enrollments != nil && s.deps.Tunnels != nil {
			api.Get("/agent/connect", s.handleAgentConnect)
		}
		// Install-artifact fetch is public: it is run by a tool with no browser
		// session (e.g. kubectl/curl) and is authorized solely by a single-use,
		// signed ticket. The credential lands only in the fetched body.
		if s.deps.Enrollments != nil && s.deps.ArtifactTickets != nil {
			api.Get("/connections/{id}/agent/enrollments/{enrollmentId}/artifacts/{kind}", s.handleFetchArtifact)
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
			// Self-service account management (any authenticated user).
			pr.Put("/auth/me", s.handleUpdateProfile)
			pr.Post("/auth/me/password", s.handleChangePassword)

			pr.Get("/plugins", s.handleListPlugins)
			pr.Get("/plugins/{name}", s.handleGetPlugin)

			pr.Get("/connections", s.handleListConnections)
			pr.Get("/connection-folders", s.handleListConnectionFolders)
			pr.Get("/credentials", s.handleListCredentials)
			pr.Get("/credential-kinds", s.handleListCredentialKinds)

			if s.deps.Connections != nil {
				pr.Post("/connections", s.handleCreateConnection)
				pr.Put("/connections/layout", s.handleSaveConnectionLayout)
				pr.Get("/connections/{id}", s.handleConnectionDetail)
				pr.Put("/connections/{id}", s.handleUpdateConnection)
				pr.Delete("/connections/{id}", s.handleDeleteConnection)
				pr.Get("/connections/{id}/session", s.handleConnectionSessionStatus)
				pr.Post("/connections/{id}/session", s.handleKeepaliveConnectionSession)
				pr.Delete("/connections/{id}/session", s.handleDisconnectConnectionSession)
				pr.Post("/connection-folders", s.handleCreateConnectionFolder)
				pr.Put("/connection-folders/{folderId}", s.handleUpdateConnectionFolder)
				pr.Delete("/connection-folders/{folderId}", s.handleDeleteConnectionFolder)
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

			if s.deps.Recordings != nil {
				pr.Get("/recordings", s.handleListRecordings)
				pr.Get("/recordings/{id}", s.handleGetRecording)
				pr.Get("/recordings/{id}/content", s.handleRecordingContent)
				pr.Head("/recordings/{id}/content", s.handleRecordingContent)
				pr.Delete("/recordings/{id}", s.handleDeleteRecording)
				if s.deps.Connections != nil {
					pr.Get("/connections/{id}/recordings", s.handleListConnectionRecordings)
				}
				if s.deps.Recording != nil {
					pr.Post("/connections/{id}/recordings/control", s.handleManualRecordingControl)
					pr.Post("/connections/{id}/recordings/desktop", s.handleStartDesktopRecording)
					pr.Post("/recordings/{id}/chunks", s.handleUploadChunk)
					pr.Post("/recordings/{id}/finalize", s.handleFinalizeRecording)
					pr.Post("/recordings/{id}/abort", s.handleAbortRecording)
				}
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
			pr.HandleFunc("/connections/{id}/proxy/*", s.handleConnectionProxy)
		})
	})

	// SPA: serve the embedded UI with history fallback (API already matched above).
	if !s.deps.Dev && s.deps.StaticFS != nil {
		r.NotFound(s.spaHandler())
	}
	return r
}
