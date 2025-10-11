package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerPermissionRoutes(api *gin.RouterGroup, handler *handlers.PermissionHandler, checker *permissions.Checker) {
	perms := api.Group("/permissions")
	{
		perms.GET("/registry", middleware.RequirePermission(checker, "permission.view"), handler.Registry)
		perms.GET("/my", handler.MyPermissions)
		perms.GET("/roles", middleware.RequirePermission(checker, "permission.view"), handler.ListRoles)
		perms.POST("/roles", middleware.RequirePermission(checker, "permission.manage"), handler.CreateRole)
		perms.PATCH("/roles/:id", middleware.RequirePermission(checker, "permission.manage"), handler.UpdateRole)
		perms.DELETE("/roles/:id", middleware.RequirePermission(checker, "permission.manage"), handler.DeleteRole)
		perms.POST("/roles/:id/permissions", middleware.RequirePermission(checker, "permission.manage"), handler.SetRolePermissions)
	}
}
