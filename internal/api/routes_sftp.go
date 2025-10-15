package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerSFTPRoutes(api *gin.RouterGroup, handler *handlers.SFTPHandler) {
	if handler == nil {
		return
	}

	group := api.Group("/active-sessions/:sessionID/sftp")
	group.GET("/list", handler.List)
	group.GET("/metadata", handler.Metadata)
	group.GET("/file", handler.ReadFile)
	group.GET("/download", handler.Download)
	group.POST("/upload", handler.Upload)
	group.PUT("/file", handler.SaveFile)
	group.POST("/rename", handler.Rename)
	group.DELETE("/file", handler.DeleteFile)
	group.DELETE("/directory", handler.DeleteDirectory)
}
