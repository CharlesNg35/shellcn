package api

import "github.com/gin-gonic/gin"

import "github.com/charlesng35/shellcn/internal/handlers"

func registerSnippetRoutes(r *gin.RouterGroup, handler *handlers.SnippetHandler) {
	if r == nil || handler == nil {
		return
	}

	snippets := r.Group("/snippets")
	{
		snippets.GET("", handler.List)
		snippets.POST("", handler.Create)
		snippets.PUT(":id", handler.Update)
		snippets.DELETE(":id", handler.Delete)
	}

	r.POST("/active-sessions/:sessionID/snippet", handler.Execute)
}
