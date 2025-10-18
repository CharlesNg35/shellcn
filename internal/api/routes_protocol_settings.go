package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerProtocolSettingsRoutes(api *gin.RouterGroup, handler *handlers.ProtocolSettingsHandler) {
	if api == nil || handler == nil {
		return
	}

	api.GET("/settings/protocols/ssh", handler.GetSSHSettings)
	api.PUT("/settings/protocols/ssh", handler.UpdateSSHSettings)
}
