package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerProtocolRoutes(api *gin.RouterGroup, handler *handlers.ProtocolHandler, checker *permissions.Checker) {
	group := api.Group("/protocols")
	{
		group.GET("", middleware.RequirePermission(checker, "connection.view"), handler.ListAll)
		group.GET("/available", middleware.RequirePermission(checker, "connection.view"), handler.ListForUser)
		group.GET("/:id/permissions", middleware.RequirePermission(checker, "connection.view"), handler.ListPermissions)
	}
}
