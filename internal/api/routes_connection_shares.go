package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerConnectionShareRoutes(api *gin.RouterGroup, handler *handlers.ConnectionShareHandler, checker *permissions.Checker) {
	shares := api.Group("/connections/:id/shares")
	{
		shares.GET("", middleware.RequirePermission(checker, "connection.share"), handler.List)
		shares.POST("", middleware.RequirePermission(checker, "connection.share"), handler.Create)
		shares.DELETE("/:shareId", middleware.RequirePermission(checker, "connection.share"), handler.Delete)
	}
}
