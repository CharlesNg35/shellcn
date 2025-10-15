package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSessionRecordingRoutes(api *gin.RouterGroup, handler *handlers.SessionRecordingHandler) {
	if api == nil || handler == nil {
		return
	}

	api.GET("/active-sessions/:sessionID/recording/status", handler.Status)
	api.POST("/active-sessions/:sessionID/recording/stop", handler.Stop)
	api.GET("/session-records/:recordID/download", handler.Download)
}
