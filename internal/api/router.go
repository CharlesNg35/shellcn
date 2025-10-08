package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/services"
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
	r.Use(middleware.CSRF())
	// Basic rate limiting: 100 requests/minute per IP+path
	r.Use(middleware.RateLimit(rateStore, 100, time.Minute))

	registerHealthRoutes(r, db)

	encryptionKey := []byte(cfg.Vault.EncryptionKey)
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

	ssoHandler := handlers.NewSSOHandler(providerRegistry, authProviderSvc, ssoManager, stateCodec)
	authProviderHandler := handlers.NewAuthProviderHandler(authProviderSvc)
	authHandler := handlers.NewAuthHandler(db, jwt, sessions, authProviderSvc, ssoManager)

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
	})

	userHandler, err := handlers.NewUserHandler(db)
	if err != nil {
		return nil, err
	}
	registerUserRoutes(api, userHandler, checker)

	orgHandler, err := handlers.NewOrganizationHandler(db)
	if err != nil {
		return nil, err
	}
	teamHandler, err := handlers.NewTeamHandler(db)
	if err != nil {
		return nil, err
	}
	registerOrganizationRoutes(api, orgHandler, teamHandler, checker)

	// Permissions
	permHandler, err := handlers.NewPermissionHandler(db)
	if err != nil {
		return nil, err
	}
	registerPermissionRoutes(api, permHandler, checker)

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

	// NotFound fallback
	r.NoRoute(middleware.NotFoundHandler)

	return r, nil
}
