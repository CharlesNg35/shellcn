package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
	appValidator "github.com/charlesng35/shellcn/pkg/validator"
)

// bindAndValidate binds the JSON payload into dest and runs struct validation rules.
// When validation fails, an error response is automatically written and false is returned.
func bindAndValidate[T any](c *gin.Context, dest *T) bool {
	if err := c.ShouldBindJSON(dest); err != nil {
		response.Error(c, appErrors.NewBadRequest("invalid JSON payload"))
		return false
	}

	if err := appValidator.ValidateStruct(dest); err != nil {
		response.Error(c, appErrors.NewBadRequest(formatValidationError(err)))
		return false
	}

	return true
}

func formatValidationError(err error) string {
	if err == nil {
		return "invalid request payload"
	}

	if ve, ok := err.(appValidator.ValidationErrors); ok {
		if len(ve) == 0 {
			return "invalid request payload"
		}

		messages := make([]string, 0, len(ve))
		for _, failure := range ve {
			field := prettifyFieldName(failure.Field)
			switch failure.Tag {
			case "required":
				messages = append(messages, fmt.Sprintf("%s is required", field))
			case "email":
				messages = append(messages, fmt.Sprintf("%s must be a valid email address", field))
			case "min":
				messages = append(messages, fmt.Sprintf("%s must be at least %s characters", field, failure.Param))
			case "max":
				messages = append(messages, fmt.Sprintf("%s must be at most %s characters", field, failure.Param))
			case "uuid4":
				messages = append(messages, fmt.Sprintf("%s must be a valid UUID", field))
			default:
				if failure.Param != "" {
					messages = append(messages, fmt.Sprintf("%s failed validation: %s=%s", field, failure.Tag, failure.Param))
				} else {
					messages = append(messages, fmt.Sprintf("%s failed validation: %s", field, failure.Tag))
				}
			}
		}
		return strings.Join(messages, "; ")
	}

	return "invalid request payload"
}

func prettifyFieldName(name string) string {
	if name == "" {
		return "field"
	}
	name = strings.ReplaceAll(name, "_", " ")
	return strings.ToLower(name)
}

func parseIntQuery(c *gin.Context, key string, fallback int) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
