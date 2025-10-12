package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Config represents the runtime configuration for the ShellCN backend.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Vault      VaultConfig      `mapstructure:"vault"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Features   FeatureConfig    `mapstructure:"features"`
	Protocols  ProtocolConfig   `mapstructure:"protocols"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Email      EmailConfig      `mapstructure:"email"`
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	Port     int        `mapstructure:"port"`
	LogLevel string     `mapstructure:"log_level"`
	CSRF     CSRFConfig `mapstructure:"csrf"`
}

// CSRFConfig controls CSRF protection middleware.
type CSRFConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// DatabaseConfig describes connection options for the supported databases.
type DatabaseConfig struct {
	Driver   string       `mapstructure:"driver"`
	Path     string       `mapstructure:"path"`
	DSN      string       `mapstructure:"dsn"`
	Postgres DBAuthConfig `mapstructure:"postgres"`
	MySQL    DBAuthConfig `mapstructure:"mysql"`
}

// CacheConfig describes cache backends.
type CacheConfig struct {
	Redis RedisCacheConfig `mapstructure:"redis"`
}

// RedisCacheConfig holds Redis connection options.
type RedisCacheConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Address  string        `mapstructure:"address"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	DB       int           `mapstructure:"db"`
	TLS      bool          `mapstructure:"tls"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// DBAuthConfig represents host based database parameters.
type DBAuthConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// VaultConfig documents encryption requirements for stored secrets.
type VaultConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
	Algorithm     string `mapstructure:"algorithm"`
}

// MonitoringConfig enables health checks and metrics.
type MonitoringConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Health     HealthConfig     `mapstructure:"health_check"`
}

// PrometheusConfig toggles metrics endpoints.
type PrometheusConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
}

// HealthConfig toggles health endpoints.
type HealthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// FeatureConfig toggles optional platform features.
type FeatureConfig struct {
	SessionSharing SessionSharingConfig `mapstructure:"session_sharing"`
	Notifications  NotificationConfig   `mapstructure:"notifications"`
}

// SessionSharingConfig controls collaborative session sharing.
type SessionSharingConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	MaxSharedUsers int  `mapstructure:"max_shared_users"`
}

// NotificationConfig toggles notifications.
type NotificationConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// ProtocolConfig enables individual protocol drivers.
type ProtocolConfig struct {
	SSH           SimpleProtocolConfig   `mapstructure:"ssh"`
	Telnet        SimpleProtocolConfig   `mapstructure:"telnet"`
	SFTP          SimpleProtocolConfig   `mapstructure:"sftp"`
	RDP           SimpleProtocolConfig   `mapstructure:"rdp"`
	VNC           SimpleProtocolConfig   `mapstructure:"vnc"`
	Docker        SimpleProtocolConfig   `mapstructure:"docker"`
	Kubernetes    SimpleProtocolConfig   `mapstructure:"kubernetes"`
	Database      DatabaseProtocolConfig `mapstructure:"database"`
	Proxmox       SimpleProtocolConfig   `mapstructure:"proxmox"`
	ObjectStorage SimpleProtocolConfig   `mapstructure:"object_storage"`
}

// SimpleProtocolConfig enables optional protocols without extra settings.
type SimpleProtocolConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// DatabaseProtocolConfig toggles database client support.
type DatabaseProtocolConfig struct {
	Enabled  bool `mapstructure:"enabled"`
	MySQL    bool `mapstructure:"mysql"`
	Postgres bool `mapstructure:"postgres"`
	Redis    bool `mapstructure:"redis"`
	MongoDB  bool `mapstructure:"mongodb"`
}

// AuthConfig captures all authentication-related settings.
type AuthConfig struct {
	JWT     JWTSettings       `mapstructure:"jwt"`
	Session SessionSettings   `mapstructure:"session"`
	Local   LocalAuthSettings `mapstructure:"local"`
}

// EmailConfig captures outbound email settings.
type EmailConfig struct {
	SMTP SMTPConfig `mapstructure:"smtp"`
}

// SMTPConfig defines SMTP dialer settings for sending email.
type SMTPConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Host     string        `mapstructure:"host"`
	Port     int           `mapstructure:"port"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	From     string        `mapstructure:"from"`
	UseTLS   bool          `mapstructure:"use_tls"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// JWTSettings configures JWT access tokens.
type JWTSettings struct {
	Secret string        `mapstructure:"secret"`
	Issuer string        `mapstructure:"issuer"`
	TTL    time.Duration `mapstructure:"access_token_ttl"`
}

// SessionSettings configures refresh tokens and session lifetimes.
type SessionSettings struct {
	RefreshTTL    time.Duration `mapstructure:"refresh_token_ttl"`
	RefreshLength int           `mapstructure:"refresh_token_length"`
}

// LocalAuthSettings defines controls for the local auth provider.
type LocalAuthSettings struct {
	LockoutThreshold int           `mapstructure:"lockout_threshold"`
	LockoutDuration  time.Duration `mapstructure:"lockout_duration"`
}

// LoadConfig initialises application configuration using Viper with sensible defaults.
func LoadConfig(paths ...string) (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	v.AddConfigPath("./config")
	for _, path := range paths {
		v.AddConfigPath(path)
	}

	setDefaults(v)

	v.SetEnvPrefix("SHELLCN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var cfgErr viper.ConfigFileNotFoundError
		if !errors.As(err, &cfgErr) {
			return nil, fmt.Errorf("config: read file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config, decodeHook()); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &config, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8000)
	v.SetDefault("server.log_level", "info")
	v.SetDefault("server.csrf.enabled", false)

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "./data/shellcn.sqlite")

	v.SetDefault("cache.redis.enabled", false)
	v.SetDefault("cache.redis.address", "127.0.0.1:6379")
	v.SetDefault("cache.redis.username", "")
	v.SetDefault("cache.redis.password", "")
	v.SetDefault("cache.redis.db", 0)
	v.SetDefault("cache.redis.tls", false)
	v.SetDefault("cache.redis.timeout", "5s")

	v.SetDefault("vault.algorithm", "aes-256-gcm")

	v.SetDefault("monitoring.prometheus.enabled", true)
	v.SetDefault("monitoring.prometheus.endpoint", "/metrics")
	v.SetDefault("monitoring.health_check.enabled", true)

	v.SetDefault("features.session_sharing.enabled", true)
	v.SetDefault("features.session_sharing.max_shared_users", 5)
	v.SetDefault("features.notifications.enabled", true)

	v.SetDefault("protocols.ssh.enabled", true)
	v.SetDefault("protocols.telnet.enabled", true)
	v.SetDefault("protocols.sftp.enabled", true)
	v.SetDefault("protocols.rdp.enabled", true)
	v.SetDefault("protocols.vnc.enabled", true)
	v.SetDefault("protocols.docker.enabled", true)
	v.SetDefault("protocols.kubernetes.enabled", false)
	v.SetDefault("protocols.database.enabled", true)
	v.SetDefault("protocols.database.mysql", true)
	v.SetDefault("protocols.database.postgres", true)
	v.SetDefault("protocols.database.redis", true)
	v.SetDefault("protocols.database.mongodb", true)
	v.SetDefault("protocols.proxmox.enabled", false)
	v.SetDefault("protocols.object_storage.enabled", false)

	v.SetDefault("auth.jwt.access_token_ttl", "15m")
	v.SetDefault("auth.session.refresh_token_ttl", "720h") // 30 days
	v.SetDefault("auth.session.refresh_token_length", 48)
	v.SetDefault("auth.local.lockout_threshold", 5)
	v.SetDefault("auth.local.lockout_duration", "15m")

	v.SetDefault("email.smtp.enabled", false)
	v.SetDefault("email.smtp.host", "")
	v.SetDefault("email.smtp.port", 587)
	v.SetDefault("email.smtp.use_tls", true)
	v.SetDefault("email.smtp.timeout", "10s")
}

func decodeHook() viper.DecoderConfigOption {
	return func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "mapstructure"
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		)
	}
}
