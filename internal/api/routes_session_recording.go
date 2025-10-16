package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerSessionRecordingRoutes(api *gin.RouterGroup, handler *handlers.SessionRecordingHandler, checker *permissions.Checker) {
	if api == nil || handler == nil {
		return
	}

	records := api.Group("/session-records")
	if checker != nil {
		records.GET("", middleware.RequirePermission(checker, "session.recording.view"), handler.List)
		records.DELETE("/:recordID", middleware.RequirePermission(checker, "session.recording.delete"), handler.Delete)
		records.GET("/:recordID/download", middleware.RequirePermission(checker, "session.recording.view"), handler.Download)
	} else {
		records.GET("", handler.List)
		records.DELETE("/:recordID", handler.Delete)
		records.GET("/:recordID/download", handler.Download)
	}

	api.GET("/active-sessions/:sessionID/recording/status", handler.Status)
	api.POST("/active-sessions/:sessionID/recording/stop", handler.Stop)
}
