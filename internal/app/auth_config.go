package app

import (
	"time"

	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/auth/providers"
)

const (
	defaultLockoutThreshold = 5
	defaultLockoutDuration  = 15 * time.Minute
)

// JWTServiceConfig converts AuthConfig into the parameters expected by the JWT service.
func (c AuthConfig) JWTServiceConfig() auth.JWTConfig {
	ttl := c.JWT.TTL
	if ttl <= 0 {
		ttl = auth.DefaultAccessTokenTTL
	}

	return auth.JWTConfig{
		Secret:         c.JWT.Secret,
		Issuer:         c.JWT.Issuer,
		AccessTokenTTL: ttl,
	}
}

// SessionServiceConfig converts AuthConfig into SessionService parameters.
func (c AuthConfig) SessionServiceConfig() auth.SessionConfig {
	ttl := c.Session.RefreshTTL
	if ttl <= 0 {
		ttl = auth.DefaultRefreshTokenTTL
	}

	length := c.Session.RefreshLength
	if length <= 0 {
		length = 48
	}

	return auth.SessionConfig{
		RefreshTokenTTL: ttl,
		RefreshLength:   length,
	}
}

// LocalProviderConfig converts AuthConfig into LocalProvider parameters.
func (c AuthConfig) LocalProviderConfig() providers.LocalConfig {
	duration := c.Local.LockoutDuration
	if duration <= 0 {
		duration = defaultLockoutDuration
	}

	threshold := c.Local.LockoutThreshold
	if threshold <= 0 {
		threshold = defaultLockoutThreshold
	}

	return providers.LocalConfig{
		LockoutThreshold: threshold,
		LockoutDuration:  duration,
	}
}
