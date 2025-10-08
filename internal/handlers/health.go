package handlers

import (
	"net/http"

	"github.com/charlesng35/shellcn/pkg/response"
	"github.com/gin-gonic/gin"
)

// Health returns a simple status payload useful for readiness checks.
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, http.StatusOK, gin.H{"status": "ok"})
	}
}
