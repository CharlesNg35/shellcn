package database

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openMySQL(cfg Config) (*gorm.DB, error) {
	dsn, err := buildMySQLDSN(cfg)
	if err != nil {
		return nil, err
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

func buildMySQLDSN(cfg Config) (string, error) {
	if cfg.DSN != "" {
		return cfg.DSN, nil
	}

	if cfg.User == "" || cfg.Name == "" {
		return "", errors.New("mysql configuration requires user and database name")
	}

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}

	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	user := cfg.User
	if cfg.Password != "" {
		user = fmt.Sprintf("%s:%s", cfg.User, cfg.Password)
	}

	baseOptions := map[string]string{
		"charset":   "utf8mb4",
		"parseTime": "True",
		"loc":       "Local",
	}

	for key, value := range cfg.Options {
		baseOptions[key] = value
	}

	keys := make([]string, 0, len(baseOptions))
	for key := range baseOptions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	opts := make([]string, 0, len(keys))
	for _, key := range keys {
		opts = append(opts, fmt.Sprintf("%s=%s", key, baseOptions[key]))
	}

	return fmt.Sprintf("%s@tcp(%s:%d)/%s?%s", user, host, port, cfg.Name, strings.Join(opts, "&")), nil
}
