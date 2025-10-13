package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerMonitoringRoutes(api *gin.RouterGroup, handler *handlers.MonitoringHandler, checker *permissions.Checker) {
	if api == nil || handler == nil || checker == nil {
		return
	}

	group := api.Group("/monitoring")
	group.GET("/summary", middleware.RequirePermission(checker, "monitoring.view"), handler.Summary)
}
