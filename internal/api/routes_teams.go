package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerTeamRoutes(api *gin.RouterGroup, teamHandler *handlers.TeamHandler, checker *permissions.Checker) {
	teams := api.Group("/teams")
	{
		teams.GET("", middleware.RequirePermission(checker, "team.view"), teamHandler.List)
		teams.GET("/:id", middleware.RequirePermission(checker, "team.view"), teamHandler.Get)
		teams.GET("/:id/roles", middleware.RequirePermission(checker, "team.view"), teamHandler.ListRoles)
		teams.GET("/:id/connections", middleware.RequirePermission(checker, "team.view"), teamHandler.ListConnections)
		teams.GET("/:id/capabilities", middleware.RequirePermission(checker, "team.view"), teamHandler.Capabilities)
		teams.GET("/:id/folders", middleware.RequirePermission(checker, "team.view"), teamHandler.ListConnectionFolders)
		teams.POST("", middleware.RequirePermission(checker, "team.create"), teamHandler.Create)
		teams.PATCH("/:id", middleware.RequirePermission(checker, "team.update"), teamHandler.Update)
		teams.DELETE("/:id", middleware.RequirePermission(checker, "team.delete"), teamHandler.Delete)
		teams.PUT("/:id/roles", middleware.RequirePermission(checker, "permission.manage"), teamHandler.SetRoles)
		teams.POST("/:id/members", middleware.RequirePermission(checker, "team.member.add"), teamHandler.AddMember)
		teams.DELETE("/:id/members/:userID", middleware.RequirePermission(checker, "team.member.remove"), teamHandler.RemoveMember)
		teams.GET("/:id/members", middleware.RequirePermission(checker, "team.view"), teamHandler.ListMembers)
	}
}
