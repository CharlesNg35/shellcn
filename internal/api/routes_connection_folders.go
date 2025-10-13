package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerConnectionFolderRoutes(api *gin.RouterGroup, handler *handlers.ConnectionFolderHandler, checker *permissions.Checker) {
	group := api.Group("/connection-folders")
	{
		group.GET("/tree", middleware.RequirePermission(checker, "connection.folder.view"), handler.ListTree)
		group.POST("", middleware.RequirePermission(checker, "connection.folder.create"), handler.Create)
		group.PATCH("/:id", middleware.RequirePermission(checker, "connection.folder.update"), handler.Update)
		group.DELETE("/:id", middleware.RequirePermission(checker, "connection.folder.delete"), handler.Delete)
	}
}
