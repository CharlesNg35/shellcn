package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/app"
	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/handlers"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/security"
)

func registerAuditRoutes(api *gin.RouterGroup, db *gorm.DB, jwt *iauth.JWTService, cfg *app.Config, checker *permissions.Checker) error {
	auditHandler, err := handlers.NewAuditHandler(db)
	if err != nil {
		return err
	}

	securitySvc := security.NewAuditService(db, jwt, cfg)
	securityHandler, err := handlers.NewSecurityHandler(securitySvc)
	if err != nil {
		return err
	}

	sec := api.Group("/security")
	{
		sec.GET("/audit", middleware.RequirePermission(checker, "security.audit"), securityHandler.Audit)
	}
	api.GET("/audit", middleware.RequirePermission(checker, "audit.view"), auditHandler.List)
	api.GET("/audit/export", middleware.RequirePermission(checker, "audit.export"), auditHandler.Export)

	return nil
}
