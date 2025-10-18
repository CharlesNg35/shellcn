package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSessionChatRoutes(api *gin.RouterGroup, handler *handlers.SessionChatHandler) {
	if handler == nil {
		return
	}

	sessions := api.Group("/active-sessions")
	sessions.GET("/:sessionID/chat", handler.ListMessages)
	sessions.POST("/:sessionID/chat", handler.PostMessage)
}
