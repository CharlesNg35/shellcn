package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerConnectionSessionRoutes(api *gin.RouterGroup, handler *handlers.ActiveConnectionHandler, checker *permissions.Checker) {
	if handler == nil {
		return
	}

	connections := api.Group("/connections")
	connections.GET("/active", middleware.RequirePermission(checker, "connection.view"), handler.ListActive)
}

func registerActiveSessionLaunchRoutes(api *gin.RouterGroup, handler *handlers.ActiveSessionLaunchHandler) {
	if handler == nil {
		return
	}
	api.POST("/active-sessions", handler.Launch)
}
