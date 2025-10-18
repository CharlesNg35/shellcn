package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSessionParticipantRoutes(api *gin.RouterGroup, handler *handlers.SessionParticipantHandler) {
	if handler == nil {
		return
	}

	group := api.Group("/active-sessions/:sessionID/participants")
	group.GET("", handler.ListParticipants)
	group.POST("", handler.AddParticipant)
	group.DELETE("/:userID", handler.RemoveParticipant)
	group.POST("/:userID/write", handler.GrantWrite)
	group.DELETE("/:userID/write", handler.RelinquishWrite)
}
