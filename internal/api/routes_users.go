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
		users.GET("/:id", middleware.RequirePermission(checker, "user.view"), handler.Get)
		users.POST("", middleware.RequirePermission(checker, "user.create"), handler.Create)
	}
}
