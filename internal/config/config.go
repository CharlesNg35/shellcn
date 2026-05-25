// Package config loads bootstrap settings (needed before the database opens)
// from a YAML file overlaid by SHELLCN_* environment variables. Settings an
// admin edits at runtime live in the store, not here.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	// Auto-load a .env file when present (local development convenience).
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"

	"github.com/charlesng/shellcn/internal/secrets"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Secrets  SecretsConfig  `mapstructure:"secrets"`
}

type ServerConfig struct {
	Addr     string `mapstructure:"addr"`
	LogLevel string `mapstructure:"log_level"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"` // sqlite | postgres | mysql
	DSN    string `mapstructure:"dsn"`    // sqlite: file path; others: connection string
}

type SecretsConfig struct {
	MasterKey     string `mapstructure:"master_key"`
	MasterKeyFile string `mapstructure:"master_key_file"`
}

// Load reads config.yaml from the current directory, ./config, or any extra
// paths, then overlays SHELLCN_* environment variables. A missing file is fine.
func Load(paths ...string) (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	for _, p := range paths {
		if p != "" {
			v.AddConfigPath(p)
		}
	}

	setDefaults(v)

	v.SetEnvPrefix("SHELLCN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	// Keep the historical master-key variable names working.
	_ = v.BindEnv("secrets.master_key", secrets.EnvMasterKey)
	_ = v.BindEnv("secrets.master_key_file", secrets.EnvMasterKeyFile)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.log_level", "info")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "shellcn.db")
}

func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.Server.LogLevel)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
