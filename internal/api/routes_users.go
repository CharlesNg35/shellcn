package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerUserRoutes(api *gin.RouterGroup, handler *handlers.UserHandler, checker *permissions.Checker) {
	users := api.Group("/users")
	{
		users.GET("", middleware.RequirePermission(checker, "user.view"), handler.List)
		users.POST("", middleware.RequirePermission(checker, "user.create"), handler.Create)
		users.POST("/bulk/activate", middleware.RequirePermission(checker, "user.activate"), handler.BulkActivate)
		users.POST("/bulk/deactivate", middleware.RequirePermission(checker, "user.deactivate"), handler.BulkDeactivate)
		users.DELETE("/bulk", middleware.RequirePermission(checker, "user.delete"), handler.BulkDelete)
		users.GET("/:id", middleware.RequirePermission(checker, "user.view"), handler.Get)
		users.PATCH("/:id", middleware.RequirePermission(checker, "user.update"), handler.Update)
		users.DELETE("/:id", middleware.RequirePermission(checker, "user.delete"), handler.Delete)
		users.POST("/:id/activate", middleware.RequirePermission(checker, "user.activate"), handler.Activate)
		users.POST("/:id/deactivate", middleware.RequirePermission(checker, "user.deactivate"), handler.Deactivate)
		users.POST("/:id/password", middleware.RequirePermission(checker, "user.password.reset"), handler.ChangePassword)
		users.PUT("/:id/roles", middleware.RequirePermission(checker, "permission.manage"), handler.SetRoles)
	}
}
