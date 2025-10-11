package database

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openPostgres(cfg Config) (*gorm.DB, error) {
	dsn, err := buildPostgresDSN(cfg)
	if err != nil {
		return nil, err
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func buildPostgresDSN(cfg Config) (string, error) {
	if cfg.DSN != "" {
		return cfg.DSN, nil
	}

	if cfg.User == "" || cfg.Name == "" {
		return "", errors.New("postgres configuration requires user and database name")
	}

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}

	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	params := []string{
		fmt.Sprintf("host=%s", host),
		fmt.Sprintf("port=%d", port),
		fmt.Sprintf("user=%s", cfg.User),
		fmt.Sprintf("dbname=%s", cfg.Name),
	}

	if cfg.Password != "" {
		params = append(params, fmt.Sprintf("password=%s", cfg.Password))
	}

	options := map[string]string{}
	for key, value := range cfg.Options {
		options[key] = value
	}

	if _, ok := options["sslmode"]; !ok {
		options["sslmode"] = "disable"
	}

	if len(options) > 0 {
		keys := make([]string, 0, len(options))
		for key := range options {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			params = append(params, fmt.Sprintf("%s=%s", key, options[key]))
		}
	}

	return strings.Join(params, " "), nil
}
