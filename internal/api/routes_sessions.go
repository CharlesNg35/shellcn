package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSessionRoutes(api *gin.RouterGroup, handler *handlers.SessionHandler) {
	api.GET("/sessions/me", handler.ListMySessions)
	api.POST("/sessions/revoke/:id", handler.Revoke)
	api.POST("/sessions/revoke_all", handler.RevokeAll)
}
