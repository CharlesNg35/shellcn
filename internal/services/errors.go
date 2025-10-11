package services

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
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

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "23505" {
		return true
	}

	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && myErr != nil && myErr.Number == 1062 {
		return true
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") ||
		strings.Contains(lower, "duplicate") ||
		strings.Contains(lower, "constraint")
}
