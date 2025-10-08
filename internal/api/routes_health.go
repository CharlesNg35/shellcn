package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/handlers"
)

func registerHealthRoutes(r *gin.Engine, db *gorm.DB) {
	r.GET("/health", handlers.Health(db))
}
