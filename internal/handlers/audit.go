package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/services"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

type AuditHandler struct {
	svc *services.AuditService
}

func NewAuditHandler(db *gorm.DB) (*AuditHandler, error) {
	svc, err := services.NewAuditService(db)
	if err != nil {
		return nil, err
	}
	return &AuditHandler{svc: svc}, nil
}

// GET /api/audit
func (h *AuditHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	per, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))

	var filters services.AuditFilters
	filters.UserID = c.Query("user_id")
	filters.Actor = c.Query("actor")
	filters.Action = c.Query("action")
	filters.Result = c.Query("result")
	filters.Resource = c.Query("resource")

	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			filters.Since = &t
		}
	}
	if u := c.Query("until"); u != "" {
		if t, err := time.Parse(time.RFC3339, u); err == nil {
			filters.Until = &t
		}
	}

	logs, total, err := h.svc.List(requestContext(c), services.AuditListOptions{Page: page, PageSize: per, Filters: filters})
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.SuccessWithMeta(c, http.StatusOK, logs, &response.Meta{Page: page, PerPage: per, Total: int(total)})
}

// GET /api/audit/export
func (h *AuditHandler) Export(c *gin.Context) {
	var filters services.AuditFilters
	filters.UserID = c.Query("user_id")
	filters.Actor = c.Query("actor")
	filters.Action = c.Query("action")
	filters.Result = c.Query("result")
	filters.Resource = c.Query("resource")

	logs, err := h.svc.Export(requestContext(c), filters)
	if err != nil {
		response.Error(c, errors.ErrInternalServer)
		return
	}

	response.Success(c, http.StatusOK, logs)
}
