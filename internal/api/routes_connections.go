package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerConnectionRoutes(api *gin.RouterGroup, handler *handlers.ConnectionHandler, checker *permissions.Checker) {
	connections := api.Group("/connections")
	{
		connections.GET("", middleware.RequirePermission(checker, "connection.view"), handler.List)
		connections.POST("", middleware.RequirePermission(checker, "connection.create"), handler.Create)
		connections.GET("/summary", middleware.RequirePermission(checker, "connection.view"), handler.Summary)
		connections.GET("/:id", middleware.RequirePermission(checker, "connection.view"), handler.Get)
		connections.PUT("/:id", middleware.RequirePermission(checker, "connection.view"), handler.Update)
		connections.DELETE("/:id", middleware.RequirePermission(checker, "connection.view"), handler.Delete)
	}
}
