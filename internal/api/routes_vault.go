package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerVaultRoutes(api *gin.RouterGroup, handler *handlers.VaultHandler, checker *permissions.Checker) {
	if handler == nil {
		return
	}

	group := api.Group("/vault")
	{
		group.GET("/identities", middleware.RequirePermission(checker, "vault.view"), handler.ListIdentities)
		group.POST("/identities", middleware.RequirePermission(checker, "vault.create"), handler.CreateIdentity)
		group.GET("/identities/:id", middleware.RequirePermission(checker, "vault.view"), handler.GetIdentity)
		group.PATCH("/identities/:id", middleware.RequirePermission(checker, "vault.edit"), handler.UpdateIdentity)
		group.DELETE("/identities/:id", middleware.RequirePermission(checker, "vault.delete"), handler.DeleteIdentity)
		group.POST("/identities/:id/shares", middleware.RequirePermission(checker, "vault.share"), handler.CreateShare)
		group.DELETE("/shares/:shareId", middleware.RequirePermission(checker, "vault.share"), handler.DeleteShare)
		group.GET("/templates", middleware.RequirePermission(checker, "vault.view"), handler.ListTemplates)
	}
}
