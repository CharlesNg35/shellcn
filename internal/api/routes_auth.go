package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

type authRouteDeps struct {
	AuthHandler       *handlers.AuthHandler
	ProviderHandler   *handlers.AuthProviderHandler
	SSOHandler        *handlers.SSOHandler
	InviteHandler     *handlers.InviteHandler
	PermissionChecker *permissions.Checker
}

func registerAuthRoutes(engine *gin.Engine, api *gin.RouterGroup, deps authRouteDeps) {
	auth := engine.Group("/api/auth")
	{
		auth.POST("/login", deps.AuthHandler.Login)
		auth.POST("/refresh", deps.AuthHandler.Refresh)
		auth.GET("/providers", deps.ProviderHandler.ListPublic)
		auth.GET("/providers/:type/login", deps.SSOHandler.Begin)
		auth.GET("/providers/:type/callback", deps.SSOHandler.Callback)
		auth.GET("/providers/:type/metadata", deps.SSOHandler.Metadata)
		auth.POST("/invite/redeem", deps.InviteHandler.Redeem)
	}

	api.GET("/auth/me", deps.AuthHandler.Me)
	api.POST("/auth/logout", deps.AuthHandler.Logout)

	providers := api.Group("/auth/providers")
	{
		providers.GET("/all", middleware.RequirePermission(deps.PermissionChecker, "permission.view"), deps.ProviderHandler.ListAll)
		providers.GET("/enabled", middleware.RequirePermission(deps.PermissionChecker, "permission.view"), deps.ProviderHandler.GetEnabled)
		providers.POST("/local/settings", middleware.RequirePermission(deps.PermissionChecker, "permission.manage"), deps.ProviderHandler.UpdateLocalSettings)
		providers.POST("/ldap/sync", middleware.RequirePermission(deps.PermissionChecker, "permission.manage"), deps.ProviderHandler.SyncLDAP)
		providers.GET("/:type", middleware.RequirePermission(deps.PermissionChecker, "permission.view"), deps.ProviderHandler.Get)
		providers.POST("/:type/enable", middleware.RequirePermission(deps.PermissionChecker, "permission.manage"), deps.ProviderHandler.SetEnabled)
		providers.POST("/:type/test", middleware.RequirePermission(deps.PermissionChecker, "permission.manage"), deps.ProviderHandler.TestConnection)
		providers.POST("/:type/configure", middleware.RequirePermission(deps.PermissionChecker, "permission.manage"), deps.ProviderHandler.Configure)
	}

	invites := api.Group("/invites")
	invites.Use(middleware.RequirePermission(deps.PermissionChecker, "user.invite"))
	{
		invites.GET("", deps.InviteHandler.List)
		invites.POST("", deps.InviteHandler.Create)
		invites.DELETE("/:id", deps.InviteHandler.Delete)
	}
}
