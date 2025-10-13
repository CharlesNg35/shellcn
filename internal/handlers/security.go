package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/charlesng35/shellcn/internal/security"
	"github.com/charlesng35/shellcn/pkg/response"
)

// SecurityHandler exposes security-related endpoints such as system audits.
type SecurityHandler struct {
	audit *security.AuditService
}

// NewSecurityHandler constructs a SecurityHandler backed by the provided audit service.
func NewSecurityHandler(audit *security.AuditService) (*SecurityHandler, error) {
	if audit == nil {
		return nil, errors.New("security handler: audit service is required")
	}
	return &SecurityHandler{audit: audit}, nil
}

// GET /api/security/audit
func (h *SecurityHandler) Audit(c *gin.Context) {
	result := h.audit.Run(requestContext(c))
	response.Success(c, http.StatusOK, result)
}
