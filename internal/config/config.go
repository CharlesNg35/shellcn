// Package config loads bootstrap settings (needed before the database opens)
// from a YAML file overlaid by SHELLCN_* environment variables. Settings an
// admin edits at runtime live in the store, not here.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	// Auto-load a .env file when present (local development convenience).
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"

	"github.com/charlesng/shellcn/internal/secrets"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Secrets    SecretsConfig    `mapstructure:"secrets"`
	Email      EmailConfig      `mapstructure:"email"`
	Recordings RecordingsConfig `mapstructure:"recordings"`
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

// EmailConfig is the outbound SMTP configuration used for account invitations.
// Disabled by default; when off, invites are shared via their copyable link.
type EmailConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	From     string `mapstructure:"from"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	UseTLS   bool   `mapstructure:"use_tls"` // implicit TLS (e.g. port 465); else STARTTLS
}

// RecordingsConfig controls session-recording storage and retention. Retention
// is OFF by default (RetentionDays == 0 keeps recordings forever); an admin opts
// in by setting a positive retention here. The cleanup job only runs when
// retention is enabled.
type RecordingsConfig struct {
	Dir             string `mapstructure:"dir"`              // blob storage root directory
	RetentionDays   int    `mapstructure:"retention_days"`   // 0 = disabled (keep forever)
	CleanupInterval string `mapstructure:"cleanup_interval"` // how often to sweep expired recordings
	MaxChunkBytes   int64  `mapstructure:"max_chunk_bytes"`  // per-chunk cap for desktop uploads
}

// RetentionEnabled reports whether expiry/cleanup is active.
func (c RecordingsConfig) RetentionEnabled() bool { return c.RetentionDays > 0 }

// CleanupEvery parses CleanupInterval, falling back to a sane default.
func (c RecordingsConfig) CleanupEvery() time.Duration {
	if d, err := time.ParseDuration(c.CleanupInterval); err == nil && d > 0 {
		return d
	}
	return time.Hour
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
	v.SetDefault("email.enabled", false)
	v.SetDefault("email.port", 587)
	v.SetDefault("email.use_tls", false)
	v.SetDefault("recordings.dir", "recordings")
	v.SetDefault("recordings.retention_days", 0) // disabled: keep recordings forever
	v.SetDefault("recordings.cleanup_interval", "1h")
	v.SetDefault("recordings.max_chunk_bytes", 8<<20)
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
