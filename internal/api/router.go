package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/mfa"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/mail"
	"github.com/charlesng35/shellcn/web"
)

// NewRouter builds the Gin engine, wires middleware and registers core routes.
// Additional module routers can mount under /api in later phases.
func NewRouter(db *gorm.DB, jwt *iauth.JWTService, cfg *app.Config, sessions *iauth.SessionService, rateStore middleware.RateStore) (*gin.Engine, error) {
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

	registerHealthRoutes(r, db)

	// Decode the vault encryption key from hex/base64 to raw bytes
	encryptionKey, err := app.DecodeKey(cfg.Vault.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decode vault encryption key: %w", err)
	}
	if length := len(encryptionKey); length != 16 && length != 24 && length != 32 {
		return nil, fmt.Errorf("invalid vault encryption key length: expected 16, 24, or 32 bytes, got %d", length)
	}

	auditSvc, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}

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

	inviteSvc, err := services.NewInviteService(db, mailer, services.WithInviteBaseURL("/invite/accept"))
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

	inviteHandler := handlers.NewInviteHandler(inviteSvc, userSvcForInvites, verificationSvc)

	// Auth routes

	// Protected routes
	checker, err := permissions.NewChecker(db)
	if err != nil {
		return nil, err
	}
	requireAuth := middleware.Auth(jwt)

	api := r.Group("/api")
	api.Use(requireAuth)

	registerAuthRoutes(r, api, authRouteDeps{
		AuthHandler:       authHandler,
		ProviderHandler:   authProviderHandler,
		SSOHandler:        ssoHandler,
		PermissionChecker: checker,
		InviteHandler:     inviteHandler,
	})

	userHandler, err := handlers.NewUserHandler(db)
	if err != nil {
		return nil, err
	}
	registerUserRoutes(api, userHandler, checker)

	profileUserSvc, err := services.NewUserService(db, auditSvc)
	if err != nil {
		return nil, err
	}
	profileHandler := handlers.NewProfileHandler(profileUserSvc, totpSvc)
	registerProfileRoutes(api, profileHandler)

	// Permissions
	permHandler, err := handlers.NewPermissionHandler(db)
	if err != nil {
		return nil, err
	}
	registerPermissionRoutes(api, permHandler, checker)

	// Realtime hub + notifications
	realtimeHub := realtime.NewHub()

	notificationHandler, err := handlers.NewNotificationHandler(db, realtimeHub)
	if err != nil {
		return nil, err
	}
	registerNotificationRoutes(api, notificationHandler, checker)

	realtimeHandler := handlers.NewRealtimeHandler(
		realtimeHub,
		jwt,
		realtime.StreamNotifications,
		realtime.StreamConnectionSessions,
	)
	r.GET("/ws", realtimeHandler.Stream)
	r.GET("/ws/:stream", realtimeHandler.Stream)

	// Connections and folders
	connectionSvc, err := services.NewConnectionService(db, checker)
	if err != nil {
		return nil, err
	}
	shareSvc, err := services.NewConnectionShareService(db, checker)
	if err != nil {
		return nil, err
	}

	connectionHandler := handlers.NewConnectionHandler(connectionSvc, shareSvc)
	registerConnectionRoutes(api, connectionHandler, checker)

	// Connection Sessions
	activeSessionSvc := services.NewActiveSessionService(realtimeHub)
	activeConnectionHandler := handlers.NewActiveConnectionHandler(activeSessionSvc, checker)
	registerConnectionSessionRoutes(api, activeConnectionHandler, checker)

	// Connection Share
	shareHandler := handlers.NewConnectionShareHandler(shareSvc)
	registerConnectionShareRoutes(api, shareHandler, checker)

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

	// Protocols
	protocolSvc, err := services.NewProtocolService(db, checker)
	if err != nil {
		return nil, err
	}
	protocolHandler := handlers.NewProtocolHandler(protocolSvc)
	registerProtocolRoutes(api, protocolHandler, checker)

	// Sessions
	sessionHandler := handlers.NewSessionHandler(db, sessions)
	registerSessionRoutes(api, sessionHandler)

	// Audit
	if err := registerAuditRoutes(api, db, jwt, cfg, checker); err != nil {
		return nil, err
	}

	// Setup (public)
	setupHandler, err := handlers.NewSetupHandler(db)
	if err != nil {
		return nil, err
	}
	registerSetupRoutes(r, setupHandler)

	// Metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Serve frontend static files
	staticFiles, err := web.FS()
	if err != nil {
		return nil, fmt.Errorf("failed to load static files: %w", err)
	}
	r.Use(serveStaticFiles(staticFiles))

	// NotFound fallback (SPA - serve index.html for client-side routing)
	r.NoRoute(func(c *gin.Context) {
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
