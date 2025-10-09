package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// ProtocolHandler delivers protocol catalogue endpoints.
type ProtocolHandler struct {
	service *services.ProtocolService
}

// NewProtocolHandler constructs a ProtocolHandler instance.
func NewProtocolHandler(service *services.ProtocolService) *ProtocolHandler {
	return &ProtocolHandler{service: service}
}

// GET /api/protocols
func (h *ProtocolHandler) ListAll(c *gin.Context) {
	protocols, err := h.service.ListAll(c.Request.Context())
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"protocols": protocols, "count": len(protocols)})
}

// GET /api/protocols/available
func (h *ProtocolHandler) ListForUser(c *gin.Context) {
	userID := c.GetString(middleware.CtxUserIDKey)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	protocols, err := h.service.ListForUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}
	response.Success(c, http.StatusOK, gin.H{"protocols": protocols, "count": len(protocols)})
}
