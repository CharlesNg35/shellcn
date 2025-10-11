package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerNotificationRoutes(api *gin.RouterGroup, handler *handlers.NotificationHandler, checker *permissions.Checker) {
	group := api.Group("/notifications")
	{
		group.GET("", middleware.RequirePermission(checker, "notification.view"), handler.List)
		group.POST("/read-all", middleware.RequirePermission(checker, "notification.view"), handler.MarkAllRead)

		group.POST("", middleware.RequirePermission(checker, "notification.manage"), handler.Create)
		group.POST("/:id/read", middleware.RequirePermission(checker, "notification.view"), handler.MarkRead)
		group.POST("/:id/unread", middleware.RequirePermission(checker, "notification.view"), handler.MarkUnread)
		group.DELETE("/:id", middleware.RequirePermission(checker, "notification.view"), handler.Delete)
	}
}
