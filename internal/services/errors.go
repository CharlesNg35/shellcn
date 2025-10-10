package services

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// isUniqueConstraintError detects database uniqueness constraint violations across vendors.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate")
}
