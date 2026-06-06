// Package config loads bootstrap settings (needed before the database opens)
// from a YAML file overlaid by SHELLCN_* environment variables. Settings an
// admin edits at runtime live in the store, not here.
package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	// Auto-load a .env file when present (local development convenience).
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/secrets"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Bootstrap  BootstrapConfig  `mapstructure:"bootstrap"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Secrets    SecretsConfig    `mapstructure:"secrets"`
	Email      EmailConfig      `mapstructure:"email"`
	Audit      AuditConfig      `mapstructure:"audit"`
	Recordings RecordingsConfig `mapstructure:"recordings"`
	Plugins    PluginsConfig    `mapstructure:"plugins"`
	AI         AIConfig         `mapstructure:"ai"`
}

type ServerConfig struct {
	Addr     string `mapstructure:"addr"`
	LogLevel string `mapstructure:"log_level"`
	// LogFile writes logs to this path instead of stdout, rotated by size with a
	// few compressed backups (100MB, 7 backups, 28 days).
	LogFile string `mapstructure:"log_file"`
	// AccessLog logs one line per API request. On by default.
	AccessLog bool `mapstructure:"access_log"`
}

type AuthConfig struct {
	SessionTTL string `mapstructure:"session_ttl"`
	JWTSecret  string `mapstructure:"jwt_secret"`
}

type BootstrapConfig struct {
	AdminUsername string `mapstructure:"admin_username"`
	AdminPassword string `mapstructure:"admin_password"`
}

// SessionTTLDuration parses SessionTTL, falling back to 24 hours.
func (c AuthConfig) SessionTTLDuration() time.Duration {
	if d, err := time.ParseDuration(c.SessionTTL); err == nil && d > 0 {
		return d
	}
	return 24 * time.Hour
}

// JWTSigningKey returns a stable HMAC key. An explicit jwt_secret takes
// precedence; otherwise the key is derived from the already-required master key.
func (c AuthConfig) JWTSigningKey(masterKey []byte) []byte {
	if c.JWTSecret != "" {
		sum := sha256.Sum256([]byte(c.JWTSecret))
		return sum[:]
	}
	sum := sha256.Sum256(append([]byte(app.JWTSigningContext), masterKey...))
	return sum[:]
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

// AuditConfig controls audit writes and optional retention cleanup. Audit is
// enabled by default; retention is OFF by default (RetentionDays == 0 keeps
// audit entries forever).
type AuditConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	RetentionDays   int    `mapstructure:"retention_days"`   // 0 = disabled (keep forever)
	CleanupInterval string `mapstructure:"cleanup_interval"` // how often to sweep expired audit rows
}

// RetentionEnabled reports whether audit expiry/cleanup is active.
func (c AuditConfig) RetentionEnabled() bool { return c.RetentionDays > 0 }

// CleanupEvery parses CleanupInterval, falling back to a sane default.
func (c AuditConfig) CleanupEvery() time.Duration {
	if d, err := time.ParseDuration(c.CleanupInterval); err == nil && d > 0 {
		return d
	}
	return time.Hour
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

// PluginsConfig points at the directory scanned for out-of-tree plugin binaries.
// Empty disables external-plugin loading; a missing directory is not an error.
type PluginsConfig struct {
	Dir    string       `mapstructure:"dir"`
	Market MarketConfig `mapstructure:"market"`
}

// MarketConfig points the gateway at one or more plugin registry indexes.
type MarketConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Indexes []string `mapstructure:"indexes"`
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

	// Match the canonical variable names used by the secret loader.
	_ = v.BindEnv("secrets.master_key", secrets.EnvMasterKey)
	_ = v.BindEnv("secrets.master_key_file", secrets.EnvMasterKeyFile)

	for _, k := range []string{"ai.kind", "ai.name", "ai.base_url", "ai.api_key", "ai.model"} {
		_ = v.BindEnv(k)
	}

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
	v.SetDefault("server.addr", ":8081")
	v.SetDefault("server.log_level", "info")
	v.SetDefault("server.log_file", "")
	v.SetDefault("server.access_log", false)
	v.SetDefault("auth.session_ttl", "24h")
	v.SetDefault("auth.jwt_secret", "")
	v.SetDefault("bootstrap.admin_username", "admin")
	v.SetDefault("bootstrap.admin_password", "")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", app.DefaultDatabaseDSN)
	v.SetDefault("email.enabled", false)
	v.SetDefault("email.port", 587)
	v.SetDefault("email.use_tls", false)
	v.SetDefault("audit.enabled", true)
	v.SetDefault("audit.retention_days", 0) // disabled: keep audit entries forever
	v.SetDefault("audit.cleanup_interval", "1h")
	v.SetDefault("recordings.dir", "recordings")
	v.SetDefault("recordings.retention_days", 0) // disabled: keep recordings forever
	v.SetDefault("recordings.cleanup_interval", "1h")
	v.SetDefault("recordings.max_chunk_bytes", 8<<20)
	v.SetDefault("plugins.dir", "plugins.d")
	v.SetDefault("plugins.market.enabled", true)
	v.SetDefault("plugins.market.indexes", []string{
		"https://raw.githubusercontent.com/CharlesNg35/shellcn-plugin-registry/main/index.json",
	})
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
