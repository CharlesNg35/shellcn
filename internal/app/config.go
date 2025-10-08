package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Config represents the runtime configuration for the ShellCN backend.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Vault      VaultConfig      `mapstructure:"vault"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Features   FeatureConfig    `mapstructure:"features"`
	Modules    ModuleConfig     `mapstructure:"modules"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Email      EmailConfig      `mapstructure:"email"`
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// DatabaseConfig describes connection options for the supported databases.
type DatabaseConfig struct {
	Driver   string       `mapstructure:"driver"`
	Path     string       `mapstructure:"path"`
	DSN      string       `mapstructure:"dsn"`
	Postgres DBAuthConfig `mapstructure:"postgres"`
	MySQL    DBAuthConfig `mapstructure:"mysql"`
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
	EncryptionKey   string `mapstructure:"encryption_key"`
	Algorithm       string `mapstructure:"algorithm"`
	KeyRotationDays int    `mapstructure:"key_rotation_days"`
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
	ClipboardSync  ClipboardConfig      `mapstructure:"clipboard_sync"`
	Notifications  NotificationConfig   `mapstructure:"notifications"`
}

// SessionSharingConfig controls collaborative session sharing.
type SessionSharingConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	MaxSharedUsers int  `mapstructure:"max_shared_users"`
}

// ClipboardConfig controls clipboard synchronisation.
type ClipboardConfig struct {
	Enabled   bool `mapstructure:"enabled"`
	MaxSizeKB int  `mapstructure:"max_size_kb"`
}

// NotificationConfig toggles notifications.
type NotificationConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// ModuleConfig enables individual protocol modules.
type ModuleConfig struct {
	SSH        SSHModuleConfig      `mapstructure:"ssh"`
	Telnet     TelnetModuleConfig   `mapstructure:"telnet"`
	SFTP       SFTPModuleConfig     `mapstructure:"sftp"`
	RDP        DesktopModuleConfig  `mapstructure:"rdp"`
	VNC        DesktopModuleConfig  `mapstructure:"vnc"`
	Docker     SimpleModuleConfig   `mapstructure:"docker"`
	Kubernetes SimpleModuleConfig   `mapstructure:"kubernetes"`
	Database   DatabaseModuleConfig `mapstructure:"database"`
	Proxmox    SimpleModuleConfig   `mapstructure:"proxmox"`
	FileShare  SimpleModuleConfig   `mapstructure:"file_share"`
}

// SSHModuleConfig configures SSH/SFTP capabilities.
type SSHModuleConfig struct {
	Enabled              bool `mapstructure:"enabled"`
	DefaultPort          int  `mapstructure:"default_port"`
	SSHV1Enabled         bool `mapstructure:"ssh_v1_enabled"`
	SSHV2Enabled         bool `mapstructure:"ssh_v2_enabled"`
	AutoReconnect        bool `mapstructure:"auto_reconnect"`
	MaxReconnectAttempts int  `mapstructure:"max_reconnect_attempts"`
	KeepaliveInterval    int  `mapstructure:"keepalive_interval"`
}

// TelnetModuleConfig configures the Telnet client.
type TelnetModuleConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	DefaultPort    int  `mapstructure:"default_port"`
	AutoReconnect  bool `mapstructure:"auto_reconnect"`
	ReconnectLimit int  `mapstructure:"max_reconnect_attempts"`
}

// SFTPModuleConfig controls the SFTP module.
type SFTPModuleConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	DefaultPort int  `mapstructure:"default_port"`
}

// DesktopModuleConfig is shared between RDP and VNC modules.
type DesktopModuleConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	DefaultPort int  `mapstructure:"default_port"`
}

// SimpleModuleConfig enables optional modules without extra settings.
type SimpleModuleConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// DatabaseModuleConfig toggles database client support.
type DatabaseModuleConfig struct {
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
	v := viper.New()
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
	v.SetDefault("server.port", 8080)

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "./data/shellcn.sqlite")

	v.SetDefault("vault.algorithm", "aes-256-gcm")
	v.SetDefault("vault.key_rotation_days", 90)

	v.SetDefault("monitoring.prometheus.enabled", true)
	v.SetDefault("monitoring.prometheus.endpoint", "/metrics")
	v.SetDefault("monitoring.health_check.enabled", true)

	v.SetDefault("features.session_sharing.enabled", true)
	v.SetDefault("features.session_sharing.max_shared_users", 5)
	v.SetDefault("features.clipboard_sync.enabled", true)
	v.SetDefault("features.clipboard_sync.max_size_kb", 1024)
	v.SetDefault("features.notifications.enabled", true)

	v.SetDefault("modules.ssh.enabled", true)
	v.SetDefault("modules.ssh.default_port", 22)
	v.SetDefault("modules.ssh.ssh_v1_enabled", false)
	v.SetDefault("modules.ssh.ssh_v2_enabled", true)
	v.SetDefault("modules.ssh.auto_reconnect", true)
	v.SetDefault("modules.ssh.max_reconnect_attempts", 3)
	v.SetDefault("modules.ssh.keepalive_interval", 60)

	v.SetDefault("modules.telnet.enabled", true)
	v.SetDefault("modules.telnet.default_port", 23)
	v.SetDefault("modules.telnet.auto_reconnect", true)
	v.SetDefault("modules.telnet.max_reconnect_attempts", 3)

	v.SetDefault("modules.sftp.enabled", true)
	v.SetDefault("modules.sftp.default_port", 22)

	v.SetDefault("modules.rdp.enabled", true)
	v.SetDefault("modules.rdp.default_port", 3389)

	v.SetDefault("modules.vnc.enabled", true)
	v.SetDefault("modules.vnc.default_port", 5900)

	v.SetDefault("modules.docker.enabled", true)
	v.SetDefault("modules.kubernetes.enabled", false)
	v.SetDefault("modules.database.enabled", true)
	v.SetDefault("modules.database.mysql", true)
	v.SetDefault("modules.database.postgres", true)
	v.SetDefault("modules.database.redis", true)
	v.SetDefault("modules.database.mongodb", true)
	v.SetDefault("modules.proxmox.enabled", false)
	v.SetDefault("modules.file_share.enabled", false)

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
