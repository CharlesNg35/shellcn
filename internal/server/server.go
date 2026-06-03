// Package server is the HTTP/WS adapter for APIs, plugin routes, and the UI.
package server

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"

	"github.com/charlesng35/shellcn/internal/ai"
	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/ai/memory"
	"github.com/charlesng35/shellcn/internal/ai/modelreg"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/config"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/service"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/internal/telemetry"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Deps are the server's injected dependencies (wired once in cmd/server).
type Deps struct {
	Plugins    *plugin.Registry
	Store      *store.Store
	Sessions   *session.Manager
	Auth       auth.Authenticator
	SessionMgr *auth.SessionManager
	Tickets    *auth.TicketStore
	// ArtifactTickets guards public install-artifact fetches.
	ArtifactTickets   *auth.TicketStore
	Policy            *policy.Enforcer
	Connector         *service.Connector
	Connections       *service.ConnectionService
	Credentials       *service.CredentialService
	Enrollments       *service.EnrollmentService
	Users             *service.UserService
	TwoFactor         *service.TwoFactorService
	Invitations       *service.InvitationService
	Tunnels           *transport.Registry
	Recordings        *service.RecordingService
	Recording         *recording.Engine
	RecordingMaxChunk int64
	AI                *aiconfig.Service
	// AIGlobal is the env/config shared-AI provider.
	AIGlobal config.AIConfig
	// ModelRegistry resolves model context windows and live model lists.
	ModelRegistry *modelreg.Registry
	Audit         audit.Sink
	Metrics       *telemetry.Metrics
	Health        *telemetry.Health
	Logger        *slog.Logger

	// StaticFS is the embedded web/dist; nil in dev mode.
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
	chat         *ai.Service
	aiTurns      *aiTurnRegistry
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
	s := &Server{deps: d, loginLimiter: newRateLimiter(rate.Every(12*time.Second), 5), aiTurns: newAITurnRegistry()}

	// Build chat here because it calls back into the server route invoker.
	if d.AI != nil {
		reg := d.ModelRegistry
		if reg == nil {
			reg = modelreg.New(modelreg.WithLogger(d.Logger))
		}
		d.AI.WithModels(reg)
		mem := memory.New(d.Store.AIConversations, d.Store.AIMessages)
		s.chat = ai.New(d.AI, d.AIGlobal, d.Plugins, s, mem, reg)
	}

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

	// Observability endpoints are unauthenticated.
	if s.deps.Health != nil {
		r.Get("/healthz", s.deps.Health.Handler())
	} else {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	}
	if s.deps.Metrics != nil {
		r.Handle("/metrics", s.deps.Metrics.Handler())
	}

	r.Route("/api", func(api chi.Router) {
		// Login is public and rate-limited per IP.
		api.With(s.loginRateLimit).Post("/auth/login", s.handleLogin)
		api.With(s.loginRateLimit).Post("/auth/login/mfa", s.handleLoginMFA)

		// Agent connect authenticates with its enrollment token.
		if s.deps.Enrollments != nil && s.deps.Tunnels != nil {
			api.Get("/agent/connect", s.handleAgentConnect)
		}
		// Install-artifact fetch uses only its single-use signed ticket.
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

			// Two-factor authentication, self-service.
			if s.deps.TwoFactor != nil {
				pr.Post("/auth/totp/setup", s.handleTOTPSetup)
				pr.Post("/auth/totp/enable", s.handleTOTPEnable)
				pr.Post("/auth/totp/disable", s.handleTOTPDisable)
				pr.Post("/auth/totp/recovery-codes", s.handleTOTPRecoveryCodes)
				pr.Post("/auth/totp/remind", s.handleTOTPRemind)
			}

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

			pr.Get("/audit/me", s.handleMyAudit)

			// AI exposes shared status plus owner-scoped provider CRUD.
			if s.deps.AI != nil {
				pr.Get("/ai/global", s.handleAIGlobal)
				pr.Get("/me/ai/config", s.handleListAIProviders)
				pr.Post("/me/ai/config", s.handleCreateAIProvider)
				pr.Post("/me/ai/models", s.handlePreviewAIProviderModels)
				pr.Post("/me/ai/test", s.handleTestAIProviderDraft)
				pr.Put("/me/ai/config/{id}", s.handleUpdateAIProvider)
				pr.Delete("/me/ai/config/{id}", s.handleDeleteAIProvider)
				pr.Get("/me/ai/config/{id}/models", s.handleAIProviderModels)
				pr.Post("/me/ai/config/{id}/test", s.handleTestAIProvider)
				if s.deps.Connections != nil {
					pr.Post("/connections/{id}/ai/turns", s.handleAITurn)
					pr.Post("/connections/{id}/ai/turns/{turnID}/control", s.handleAITurnControl)
					pr.Get("/connections/{id}/ai/conversations", s.handleListConversations)
					pr.Post("/connections/{id}/ai/conversations", s.handleCreateConversation)
					pr.Get("/connections/{id}/ai/conversations/{cid}", s.handleGetConversation)
					pr.Get("/connections/{id}/ai/conversations/{cid}/messages", s.handleConversationMessages)
					pr.Put("/connections/{id}/ai/conversations/{cid}", s.handleRenameConversation)
					pr.Delete("/connections/{id}/ai/conversations/{cid}", s.handleDeleteConversation)
				}
			}
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
					ar.Get("/admin/users/search", s.handleSearchUsers)
					ar.Post("/admin/users", s.handleAdminCreateUser)
					ar.Get("/admin/users/{id}", s.handleAdminGetUser)
					ar.Put("/admin/users/{id}", s.handleAdminUpdateUser)
					ar.Post("/admin/users/{id}/activate", s.handleAdminActivateUser)
					ar.Post("/admin/users/{id}/deactivate", s.handleAdminDeactivateUser)
					ar.Post("/admin/users/{id}/reset-2fa", s.handleAdminResetTwoFactor)
					ar.Get("/admin/users/{id}/audit", s.handleAdminUserAudit)
					ar.Get("/admin/users/{id}/connections", s.handleAdminUserConnections)
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

		// The connection web proxy is cookie-authenticated but CSRF-exempt: a
		// proxied third-party app cannot send our CSRF token, and SameSite=Lax
		// already blocks cross-site cookie use on non-GET requests.
		api.Group(func(xr chi.Router) {
			xr.Use(s.requireSession)
			xr.HandleFunc("/connections/{id}/proxy/*", s.handleConnectionProxy)
		})
	})

	// SPA: serve the embedded UI with history fallback (API already matched above).
	if !s.deps.Dev && s.deps.StaticFS != nil {
		r.NotFound(s.spaHandler())
	}
	return r
}
