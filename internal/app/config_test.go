package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
)

func TestLoadConfigFromFile(t *testing.T) {
	path := filepath.Join("testdata")
	cfg, err := LoadConfig(path)
	require.NoError(t, err)

	require.Equal(t, 9090, cfg.Server.Port)
	require.Equal(t, "info", cfg.Server.LogLevel)
	require.Equal(t, "postgres", cfg.Database.Driver)
	require.True(t, cfg.Database.Postgres.Enabled)
	require.Equal(t, "db.example.com", cfg.Database.Postgres.Host)
	require.Equal(t, 60, cfg.Vault.KeyRotationDays)

	require.True(t, cfg.Cache.Redis.Enabled)
	require.Equal(t, "redis.example.com:6379", cfg.Cache.Redis.Address)
	require.Equal(t, "shellcn", cfg.Cache.Redis.Username)
	require.Equal(t, "redis-secret", cfg.Cache.Redis.Password)
	require.Equal(t, 2, cfg.Cache.Redis.DB)
	require.True(t, cfg.Cache.Redis.TLS)
	require.Equal(t, 4*time.Second, cfg.Cache.Redis.Timeout)

	require.True(t, cfg.Features.SessionSharing.Enabled)
	require.Equal(t, 8, cfg.Features.SessionSharing.MaxSharedUsers)

	require.True(t, cfg.Modules.SSH.Enabled)
	require.False(t, cfg.Modules.Telnet.Enabled)
	require.True(t, cfg.Modules.SFTP.Enabled)
	require.True(t, cfg.Modules.RDP.Enabled)
	require.True(t, cfg.Modules.VNC.Enabled)
	require.True(t, cfg.Modules.Docker.Enabled)
	require.True(t, cfg.Modules.Kubernetes.Enabled)

	require.True(t, cfg.Modules.Database.Enabled)
	require.True(t, cfg.Modules.Database.MySQL)
	require.False(t, cfg.Modules.Database.Postgres)
	require.True(t, cfg.Modules.Database.Redis)
	require.True(t, cfg.Modules.Database.MongoDB)

	require.False(t, cfg.Modules.Proxmox.Enabled)
	require.True(t, cfg.Modules.ObjectStorage.Enabled)

	require.Equal(t, "jwt-secret", cfg.Auth.JWT.Secret)
	require.Equal(t, 30*time.Minute, cfg.Auth.JWT.TTL)
	require.Equal(t, 1440*time.Hour, cfg.Auth.Session.RefreshTTL)
	require.Equal(t, 64, cfg.Auth.Session.RefreshLength)
	require.Equal(t, 7, cfg.Auth.Local.LockoutThreshold)
	require.Equal(t, 20*time.Minute, cfg.Auth.Local.LockoutDuration)

	require.True(t, cfg.Email.SMTP.Enabled)
	require.Equal(t, "smtp.example.com", cfg.Email.SMTP.Host)
	require.Equal(t, 2525, cfg.Email.SMTP.Port)
	require.Equal(t, "smtp-user", cfg.Email.SMTP.Username)
	require.Equal(t, "smtp-pass", cfg.Email.SMTP.Password)
	require.Equal(t, "no-reply@example.com", cfg.Email.SMTP.From)
	require.True(t, cfg.Email.SMTP.UseTLS)
	require.Equal(t, 15*time.Second, cfg.Email.SMTP.Timeout)
}

func TestAuthConfigAdapters(t *testing.T) {
	cfg := Config{
		Auth: AuthConfig{
			JWT: JWTSettings{
				Secret: "secret",
				Issuer: "issuer",
				TTL:    30 * time.Minute,
			},
			Session: SessionSettings{
				RefreshTTL:    10 * time.Hour,
				RefreshLength: 32,
			},
			Local: LocalAuthSettings{
				LockoutThreshold: 4,
				LockoutDuration:  10 * time.Minute,
			},
		},
	}

	jwtCfg := cfg.Auth.JWTServiceConfig()
	require.Equal(t, auth.JWTConfig{
		Secret:         "secret",
		Issuer:         "issuer",
		AccessTokenTTL: 30 * time.Minute,
	}, jwtCfg)

	sessionCfg := cfg.Auth.SessionServiceConfig()
	require.Equal(t, auth.SessionConfig{
		RefreshTokenTTL: 10 * time.Hour,
		RefreshLength:   32,
	}, sessionCfg)

	localCfg := cfg.Auth.LocalProviderConfig()
	require.Equal(t, providers.LocalConfig{
		LockoutThreshold: 4,
		LockoutDuration:  10 * time.Minute,
	}, localCfg)
}

func TestAuthConfigAdaptersFallback(t *testing.T) {
	var cfg AuthConfig

	jwtCfg := cfg.JWTServiceConfig()
	require.Equal(t, auth.DefaultAccessTokenTTL, jwtCfg.AccessTokenTTL)

	sessionCfg := cfg.SessionServiceConfig()
	require.Equal(t, auth.DefaultRefreshTokenTTL, sessionCfg.RefreshTokenTTL)
	require.Equal(t, 48, sessionCfg.RefreshLength)

	localCfg := cfg.LocalProviderConfig()
	require.Equal(t, defaultLockoutThreshold, localCfg.LockoutThreshold)
	require.Equal(t, defaultLockoutDuration, localCfg.LockoutDuration)
}

func TestEmailConfigAdapter(t *testing.T) {
	cfg := EmailConfig{
		SMTP: SMTPConfig{
			Enabled:  true,
			Host:     "smtp.example.com",
			Port:     2525,
			Username: "user",
			Password: "pass",
			From:     "no-reply@example.com",
			UseTLS:   true,
			Timeout:  10 * time.Second,
		},
	}

	settings := cfg.SMTPSettings()
	require.True(t, settings.Enabled)
	require.Equal(t, "smtp.example.com", settings.Host)
	require.Equal(t, 2525, settings.Port)
	require.Equal(t, "user", settings.Username)
	require.Equal(t, "pass", settings.Password)
	require.Equal(t, "no-reply@example.com", settings.From)
	require.True(t, settings.UseTLS)
	require.Equal(t, 10*time.Second, settings.Timeout)
}
