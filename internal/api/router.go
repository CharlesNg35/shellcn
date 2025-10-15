package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/mfa"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/monitoring"
	"github.com/charlesng35/shellcn/internal/monitoring/checks"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/internal/vault"
	"github.com/charlesng35/shellcn/pkg/mail"
	"github.com/charlesng35/shellcn/web"
)

// NewRouter builds the Gin engine, wires middleware and registers core routes.
// Additional module routers can mount under /api in later phases.
func NewRouter(db *gorm.DB, jwt *iauth.JWTService, cfg *app.Config, driverReg *drivers.Registry, sessions *iauth.SessionService, rateStore middleware.RateStore, mon *monitoring.Module) (*gin.Engine, error) {
	if db == nil {
		return nil, fmt.Errorf("database handle must be provided")
	}
	if jwt == nil {
		return nil, fmt.Errorf("jwt service must be provided")
	}
	if sessions == nil {
		return nil, fmt.Errorf("session service must be provided")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config must be provided")
	}

	r := gin.New()

	// Global middleware
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Metrics())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.CORS())
	if cfg.Server.CSRF.Enabled {
		r.Use(middleware.CSRF())
	}
	// Basic rate limiting: 300 requests/minute per IP+path
	r.Use(middleware.RateLimit(rateStore, 300, time.Minute))

	registerHealthRoutes(r, cfg, mon)

	// Decode the vault encryption key from hex/base64 to raw bytes
	encryptionKey, err := app.DecodeKey(cfg.Vault.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode vault encryption key: %w", err)
	}
	if length := len(encryptionKey); length != 32 {
		return nil, fmt.Errorf("invalid vault encryption key length: expected 32 bytes, got %d", length)
	}

	auditSvc, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}

	checker, err := permissions.NewChecker(db)
	if err != nil {
		return nil, err
	}
	monitoringHandler := handlers.NewMonitoringHandler(mon, cfg)

	authProviderSvc, err := services.NewAuthProviderService(db, auditSvc, encryptionKey)
	if err != nil {
		return nil, err
	}

	totpSvc, err := mfa.NewTOTPService(db, encryptionKey)
	if err != nil {
		return nil, err
	}

	var mailer mail.Mailer
	if cfg.Email.SMTP.Enabled {
		mailer, err = mail.NewSMTPMailer(mail.SMTPSettings{
			Enabled:  cfg.Email.SMTP.Enabled,
			Host:     cfg.Email.SMTP.Host,
			Port:     cfg.Email.SMTP.Port,
			Username: cfg.Email.SMTP.Username,
			Password: cfg.Email.SMTP.Password,
			From:     cfg.Email.SMTP.From,
			UseTLS:   cfg.Email.SMTP.UseTLS,
			Timeout:  cfg.Email.SMTP.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("configure smtp mailer: %w", err)
		}
	}

	inviteSvc, err := services.NewInviteService(db, mailer,
		services.WithInviteBaseURL("/invite/accept"),
		services.WithInviteAuditService(auditSvc),
	)
	if err != nil {
		return nil, err
	}

	verificationSvc, err := services.NewEmailVerificationService(db, mailer)
	if err != nil {
		return nil, err
	}

	providerRegistry := providers.NewRegistry()
	if err := providerRegistry.Register(providers.NewOIDCDescriptor(providers.OIDCOptions{})); err != nil {
		return nil, err
	}
	if err := providerRegistry.Register(providers.NewSAMLDescriptor(providers.SAMLOptions{})); err != nil {
		return nil, err
	}

	stateCodec, err := iauth.NewStateCodec(encryptionKey, 10*time.Minute, nil)
	if err != nil {
		return nil, err
	}

	ssoManager, err := iauth.NewSSOManager(db, sessions, iauth.SSOConfig{})
	if err != nil {
		return nil, err
	}

	ldapSyncSvc, err := services.NewLDAPSyncService(db, ssoManager)
	if err != nil {
		return nil, err
	}

	ssoHandler := handlers.NewSSOHandler(providerRegistry, authProviderSvc, ssoManager, stateCodec)
	authProviderHandler := handlers.NewAuthProviderHandler(authProviderSvc, ldapSyncSvc)
	authHandler := handlers.NewAuthHandler(db, jwt, sessions, authProviderSvc, ssoManager, ldapSyncSvc, totpSvc, verificationSvc)

	userSvcForInvites, err := services.NewUserService(db, auditSvc)
	if err != nil {
		return nil, err
	}

	teamSvcForInvites, err := services.NewTeamService(db, auditSvc, checker)
	if err != nil {
		return nil, err
	}

	inviteHandler := handlers.NewInviteHandler(inviteSvc, userSvcForInvites, teamSvcForInvites, verificationSvc)

	// Auth routes

	// ----- Protected API Group --------------------------------------------------
	requireAuth := middleware.Auth(jwt)

	api := r.Group("/api")
	api.Use(requireAuth)

	// ----- Authentication & Invite Routes --------------------------------------
	registerAuthRoutes(r, api, authRouteDeps{
		AuthHandler:       authHandler,
		ProviderHandler:   authProviderHandler,
		SSOHandler:        ssoHandler,
		PermissionChecker: checker,
		InviteHandler:     inviteHandler,
		JWT:               jwt,
	})

	userHandler, err := handlers.NewUserHandler(db)
	if err != nil {
		return nil, err
	}
	// ----- User Routes ---------------------------------------------------------
	registerUserRoutes(api, userHandler, checker)
	// ----- End User Routes -----------------------------------------------------

	profileUserSvc, err := services.NewUserService(db, auditSvc)
	if err != nil {
		return nil, err
	}
	profileHandler := handlers.NewProfileHandler(profileUserSvc, totpSvc)
	// ----- Profile Routes ------------------------------------------------------
	registerProfileRoutes(api, profileHandler)
	// ----- End Profile Routes --------------------------------------------------

	permHandler, err := handlers.NewPermissionHandler(db, auditSvc)
	if err != nil {
		return nil, err
	}
	// ----- Permission Routes ---------------------------------------------------
	registerPermissionRoutes(api, permHandler, checker)
	// ----- End Permission Routes ----------------------------------------------

	// Realtime hub + notifications
	realtimeHub := realtime.NewHub()
	if mon != nil && mon.Health() != nil {
		mon.Health().RegisterReadiness(checks.Realtime(realtimeHub))
	}

	notificationHandler, err := handlers.NewNotificationHandler(db, realtimeHub)
	if err != nil {
		return nil, err
	}
	// ----- Realtime & Notification Routes -------------------------------------
	registerNotificationRoutes(api, notificationHandler, checker)
	registerMonitoringRoutes(api, monitoringHandler, checker)

	vaultCrypto, err := vault.NewCrypto(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("initialise vault crypto: %w", err)
	}
	vaultSvc, err := services.NewVaultService(db, auditSvc, checker, vaultCrypto)
	if err != nil {
		return nil, fmt.Errorf("initialise vault service: %w", err)
	}
	vaultHandler := handlers.NewVaultHandler(vaultSvc, rateStore)

	// ----- Connection, Share & Folder Routes -----------------------------------
	connectionSvc, err := services.NewConnectionService(db, checker, services.WithConnectionVault(vaultSvc))
	if err != nil {
		return nil, err
	}
	shareSvc, err := services.NewConnectionShareService(db, checker, services.WithConnectionShareVault(vaultSvc))
	if err != nil {
		return nil, err
	}

	connectionHandler := handlers.NewConnectionHandler(connectionSvc, shareSvc)
	registerConnectionRoutes(api, connectionHandler, checker)

	// Connection Sessions
	activeSessionSvc := services.NewActiveSessionService(realtimeHub)
	activeConnectionHandler := handlers.NewActiveConnectionHandler(activeSessionSvc, checker)
	registerConnectionSessionRoutes(api, activeConnectionHandler, checker)

	sessionChatSvc, err := services.NewSessionChatService(db, activeSessionSvc)
	if err != nil {
		return nil, err
	}
	sessionLifecycleSvc, err := services.NewSessionLifecycleService(
		db,
		activeSessionSvc,
		services.WithSessionAuditService(auditSvc),
		services.WithSessionChatStore(sessionChatSvc),
	)
	if err != nil {
		return nil, err
	}

	sshHandler := handlers.NewSSHSessionHandler(cfg, connectionSvc, vaultSvc, realtimeHub, activeSessionSvc, sessionLifecycleSvc, driverReg, checker, jwt)
	realtimeHandler := handlers.NewRealtimeHandler(
		realtimeHub,
		jwt,
		sshHandler,
		realtime.StreamNotifications,
		realtime.StreamConnectionSessions,
		realtime.StreamSSHTerminal,
	)
	r.GET("/ws", realtimeHandler.Stream)
	r.GET("/ws/:stream", realtimeHandler.Stream)

	// Connection Share
	shareHandler := handlers.NewConnectionShareHandler(shareSvc)
	registerConnectionShareRoutes(api, shareHandler, checker)
	registerVaultRoutes(api, vaultHandler, checker)

	folderSvc, err := services.NewConnectionFolderService(db, checker, connectionSvc)
	if err != nil {
		return nil, err
	}
	connectionFolderHandler := handlers.NewConnectionFolderHandler(folderSvc)
	registerConnectionFolderRoutes(api, connectionFolderHandler, checker)

	teamHandler, err := handlers.NewTeamHandler(db, checker, connectionSvc, folderSvc)
	if err != nil {
		return nil, err
	}
	registerTeamRoutes(api, teamHandler, checker)
	// ----- End Connection, Share & Folder Routes ------------------------------

	// ----- Protocol Routes -----------------------------------------------------
	protocolSvc, err := services.NewProtocolService(db, checker)
	if err != nil {
		return nil, err
	}
	if mon != nil && mon.Health() != nil {
		mon.Health().RegisterReadiness(monitoring.NewCheck("protocol_catalog", func(ctx context.Context) monitoring.ProbeResult {
			start := time.Now()
			protocols, err := protocolSvc.ListAll(ctx)
			if err != nil {
				return monitoring.ResultFromError("protocol_catalog", err, time.Since(start))
			}
			if len(protocols) == 0 {
				return monitoring.ProbeResult{
					Status:   monitoring.StatusUp,
					Details:  "no protocols available",
					Duration: time.Since(start),
				}
			}
			return monitoring.ProbeResult{Status: monitoring.StatusUp, Duration: time.Since(start)}
		}))
	}
	protocolHandler := handlers.NewProtocolHandler(protocolSvc)
	registerProtocolRoutes(api, protocolHandler, checker)
	// ----- End Protocol Routes -------------------------------------------------

	// ----- Session Routes ------------------------------------------------------
	sessionHandler := handlers.NewSessionHandler(db, sessions)
	registerSessionRoutes(api, sessionHandler)
	// ----- End Session Routes --------------------------------------------------

	// Audit
	if err := registerAuditRoutes(api, db, jwt, cfg, checker); err != nil {
		return nil, err
	}
	// ----- End Audit Routes ----------------------------------------------------

	// Setup (public)
	setupHandler, err := handlers.NewSetupHandler(db)
	if err != nil {
		return nil, err
	}
	registerSetupRoutes(r, setupHandler)
	// ----- End Setup Routes ----------------------------------------------------
	// ----- End Protected API Group --------------------------------------------

	metricsEndpoint := strings.TrimSpace(cfg.Monitoring.Prometheus.Endpoint)
	if metricsEndpoint == "" {
		metricsEndpoint = "/metrics"
	}
	if !strings.HasPrefix(metricsEndpoint, "/") {
		metricsEndpoint = "/" + metricsEndpoint
	}

	if mon != nil && cfg.Monitoring.Prometheus.Enabled {
		r.GET(metricsEndpoint, gin.WrapH(mon.Handler()))
	}

	// Serve frontend static files
	staticFiles, err := web.FS()
	if err != nil {
		return nil, fmt.Errorf("failed to load static files: %w", err)
	}
	r.Use(serveStaticFiles(staticFiles))

	// NotFound fallback (SPA - serve index.html for client-side routing)
	r.NoRoute(func(c *gin.Context) {
		if !cfg.Monitoring.Prometheus.Enabled && c.Request.URL.Path == metricsEndpoint {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if c.Request.URL.Path == "/metrics" && metricsEndpoint != "/metrics" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		// If the request is for an API endpoint, return 404
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			middleware.NotFoundHandler(c)
			return
		}
		// Otherwise, serve index.html for SPA routing
		c.FileFromFS("/", http.FS(staticFiles))
	})

	return r, nil
}

// serveStaticFiles returns a middleware that serves static files from the embedded filesystem.
func serveStaticFiles(staticFS fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(staticFS))
	return func(c *gin.Context) {
		// Skip if it's an API route
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.Next()
			return
		}

		// Try to serve the static file
		path := c.Request.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		if _, err := fs.Stat(staticFS, path[1:]); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		// File doesn't exist, continue to next handler (will hit NoRoute for SPA)
		c.Next()
	}
}
