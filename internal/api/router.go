package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/security"
)

// NewRouter builds the Gin engine, wires middleware and registers core routes.
// Additional module routers can mount under /api in later phases.
func NewRouter(db *gorm.DB, jwt *iauth.JWTService, cfg *app.Config, sessions *iauth.SessionService) (*gin.Engine, error) {
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
	r.Use(middleware.RateLimit(100, time.Minute))

	// Health endpoint (public)
	r.GET("/health", handlers.Health(db))

	authHandler := handlers.NewAuthHandler(db, jwt, sessions)

	// Public auth routes
	auth := r.Group("/api/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)
	}

	// Protected routes
	checker, err := permissions.NewChecker(db)
	if err != nil {
		return nil, err
	}
	requireAuth := middleware.Auth(jwt)

	api := r.Group("/api")
	api.Use(requireAuth)

	// Authenticated auth routes
	api.GET("/auth/me", authHandler.Me)
	api.POST("/auth/logout", authHandler.Logout)

	// Users
	userHandler, err := handlers.NewUserHandler(db)
	if err != nil {
		return nil, err
	}

	users := api.Group("/users")
	{
		users.GET("", middleware.RequirePermission(checker, "user.view"), userHandler.List)
		users.GET("/:id", middleware.RequirePermission(checker, "user.view"), userHandler.Get)
		users.POST("", middleware.RequirePermission(checker, "user.create"), userHandler.Create)
		// Additional handlers (update/delete) will be added in subsequent iterations
	}

	// Organizations
	orgHandler, err := handlers.NewOrganizationHandler(db)
	if err != nil {
		return nil, err
	}
	orgs := api.Group("/orgs")
	{
		orgs.GET("", middleware.RequirePermission(checker, "org.view"), orgHandler.List)
		orgs.GET("/:id", middleware.RequirePermission(checker, "org.view"), orgHandler.Get)
		orgs.POST("", middleware.RequirePermission(checker, "org.create"), orgHandler.Create)
		orgs.PATCH("/:id", middleware.RequirePermission(checker, "org.manage"), orgHandler.Update)
		orgs.DELETE("/:id", middleware.RequirePermission(checker, "org.manage"), orgHandler.Delete)
	}

	// Teams
	teamHandler, err := handlers.NewTeamHandler(db)
	if err != nil {
		return nil, err
	}
	teams := api.Group("/teams")
	{
		teams.GET("/:id", middleware.RequirePermission(checker, "org.view"), teamHandler.Get)
		teams.PATCH("/:id", middleware.RequirePermission(checker, "org.manage"), teamHandler.Update)
		teams.POST("", middleware.RequirePermission(checker, "org.manage"), teamHandler.Create)
		teams.POST("/:id/members", middleware.RequirePermission(checker, "org.manage"), teamHandler.AddMember)
		teams.DELETE("/:id/members/:userID", middleware.RequirePermission(checker, "org.manage"), teamHandler.RemoveMember)
		teams.GET("/:id/members", middleware.RequirePermission(checker, "org.view"), teamHandler.ListMembers)
	}
	api.GET("/organizations/:orgID/teams", middleware.RequirePermission(checker, "org.view"), teamHandler.ListByOrg)

	// Permissions
	permHandler, err := handlers.NewPermissionHandler(db)
	if err != nil {
		return nil, err
	}
	perms := api.Group("/permissions")
	{
		perms.GET("/registry", middleware.RequirePermission(checker, "permission.view"), permHandler.Registry)
		perms.GET("/roles", middleware.RequirePermission(checker, "permission.view"), permHandler.ListRoles)
		perms.POST("/roles", middleware.RequirePermission(checker, "permission.manage"), permHandler.CreateRole)
		perms.PATCH("/roles/:id", middleware.RequirePermission(checker, "permission.manage"), permHandler.UpdateRole)
		perms.DELETE("/roles/:id", middleware.RequirePermission(checker, "permission.manage"), permHandler.DeleteRole)
		perms.POST("/roles/:id/permissions", middleware.RequirePermission(checker, "permission.manage"), permHandler.SetRolePermissions)
	}

	// Sessions
	sessionHandler := handlers.NewSessionHandler(db, sessions)
	api.GET("/sessions/me", sessionHandler.ListMySessions)
	api.POST("/sessions/revoke/:id", sessionHandler.Revoke)
	api.POST("/sessions/revoke_all", sessionHandler.RevokeAll)

	// Audit
	auditHandler, err := handlers.NewAuditHandler(db)
	if err != nil {
		return nil, err
	}

	securitySvc := security.NewAuditService(db, jwt, cfg)
	securityHandler, err := handlers.NewSecurityHandler(securitySvc)
	if err != nil {
		return nil, err
	}
	sec := api.Group("/security")
	{
		sec.GET("/audit", middleware.RequirePermission(checker, "security.audit"), securityHandler.Audit)
	}
	api.GET("/audit", middleware.RequirePermission(checker, "audit.view"), auditHandler.List)
	api.GET("/audit/export", middleware.RequirePermission(checker, "audit.export"), auditHandler.Export)

	// Auth providers (note: encryption key should be provided from config in server wiring)
	encryptionKey := []byte(cfg.Vault.EncryptionKey)
	if length := len(encryptionKey); length != 16 && length != 24 && length != 32 {
		return nil, fmt.Errorf("invalid vault encryption key length: expected 16, 24, or 32 bytes, got %d", length)
	}

	apHandler, err := handlers.NewAuthProviderHandler(db, encryptionKey)
	if err != nil {
		return nil, err
	}
	ap := api.Group("/auth/providers")
	{
		ap.GET("", middleware.RequirePermission(checker, "permission.view"), apHandler.List)
		ap.GET("/enabled", middleware.RequirePermission(checker, "permission.view"), apHandler.GetEnabled)
		ap.POST("/local/settings", middleware.RequirePermission(checker, "permission.manage"), apHandler.UpdateLocalSettings)
		ap.POST("/:type/enable", middleware.RequirePermission(checker, "permission.manage"), apHandler.SetEnabled)
		ap.POST("/:type/test", middleware.RequirePermission(checker, "permission.manage"), apHandler.TestConnection)
		ap.POST("/:type/configure", middleware.RequirePermission(checker, "permission.manage"), apHandler.Configure)
	}

	// Setup (public)
	setupHandler, err := handlers.NewSetupHandler(db)
	if err != nil {
		return nil, err
	}
	r.GET("/api/setup/status", setupHandler.Status)
	r.POST("/api/setup/initialize", setupHandler.Initialize)

	// Metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// NotFound fallback
	r.NoRoute(middleware.NotFoundHandler)

	return r, nil
}
