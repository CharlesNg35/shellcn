package api

import (
	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func registerOrganizationRoutes(api *gin.RouterGroup, orgHandler *handlers.OrganizationHandler, teamHandler *handlers.TeamHandler, checker *permissions.Checker) {
	orgs := api.Group("/orgs")
	{
		orgs.GET("", middleware.RequirePermission(checker, "org.view"), orgHandler.List)
		orgs.GET("/:id", middleware.RequirePermission(checker, "org.view"), orgHandler.Get)
		orgs.POST("", middleware.RequirePermission(checker, "org.create"), orgHandler.Create)
		orgs.PATCH("/:id", middleware.RequirePermission(checker, "org.manage"), orgHandler.Update)
		orgs.DELETE("/:id", middleware.RequirePermission(checker, "org.manage"), orgHandler.Delete)
	}

	teams := api.Group("/teams")
	{
		teams.GET("/:id", middleware.RequirePermission(checker, "org.view"), teamHandler.Get)
		teams.PATCH("/:id", middleware.RequirePermission(checker, "org.manage"), teamHandler.Update)
		teams.POST("", middleware.RequirePermission(checker, "org.manage"), teamHandler.Create)
		teams.POST("/:id/members", middleware.RequirePermission(checker, "org.manage"), teamHandler.AddMember)
		teams.DELETE("/:id/members/:userID", middleware.RequirePermission(checker, "org.manage"), teamHandler.RemoveMember)
		teams.GET("/:id/members", middleware.RequirePermission(checker, "org.view"), teamHandler.ListMembers)
	}

	api.GET("/organizations/:orgID/teams", middleware.RequirePermission(checker, "org.view"), teamHandler.ListByOrg)
}
