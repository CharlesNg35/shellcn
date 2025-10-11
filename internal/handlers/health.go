package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/pkg/response"
)

const healthVersion = "1.0.0"

// Health returns a readiness payload indicating overall application status.
func Health(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload := gin.H{
			"status":   "healthy",
			"database": "up",
			"version":  healthVersion,
		}

		if db == nil {
			response.Success(c, http.StatusOK, payload)
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			respondHealthUnavailable(c, "error")
			return
		}

		if err := sqlDB.PingContext(requestContext(c)); err != nil {
			respondHealthUnavailable(c, "down")
			return
		}

		response.Success(c, http.StatusOK, payload)
	}
}

func respondHealthUnavailable(c *gin.Context, dbStatus string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"success":  false,
		"status":   "unhealthy",
		"database": dbStatus,
		"version":  healthVersion,
	})
}
