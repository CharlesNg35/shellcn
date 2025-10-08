package response

import (
	"net/http"

	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/gin-gonic/gin"
)

// Response defines the base API payload.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorInfo holds error details to send to clients.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Meta describes pagination metadata.
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// Success writes a JSON success response.
func Success(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, Response{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMeta writes a JSON success response including metadata.
func SuccessWithMeta(c *gin.Context, statusCode int, data interface{}, meta *Meta) {
	c.JSON(statusCode, Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// Error writes a JSON error response derived from an AppError.
func Error(c *gin.Context, err error) {
	if err == nil {
		err = appErrors.ErrInternalServer
	}

	appErr := appErrors.FromError(err)
	status := appErr.StatusCode
	if status == 0 {
		status = http.StatusInternalServerError
	}

	c.JSON(status, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    appErr.Code,
			Message: appErr.Message,
		},
	})
}
