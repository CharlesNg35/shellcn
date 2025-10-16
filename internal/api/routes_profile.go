package api

import "github.com/gin-gonic/gin"

import "github.com/charlesng35/shellcn/internal/handlers"

func registerProfileRoutes(api *gin.RouterGroup, handler *handlers.ProfileHandler) {
	profile := api.Group("/profile")
	{
		profile.PATCH("", handler.Update)
		profile.POST("/password", handler.ChangePassword)
		profile.POST("/mfa/setup", handler.SetupMFA)
		profile.POST("/mfa/enable", handler.EnableMFA)
		profile.POST("/mfa/disable", handler.DisableMFA)
		profile.GET("/preferences", handler.GetPreferences)
		profile.PUT("/preferences", handler.UpdatePreferences)
	}
}
