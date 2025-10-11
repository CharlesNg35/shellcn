package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSetupRoutes(r *gin.Engine, handler *handlers.SetupHandler) {
	r.GET("/api/setup/status", handler.Status)
	r.POST("/api/setup/initialize", handler.Initialize)
}
