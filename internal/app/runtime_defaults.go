package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/charlesng35/shellcn/pkg/crypto"
)

const (
	jwtSecretBytes   = 48
	vaultSecretBytes = 32
)

// ApplyRuntimeDefaults ensures critical secrets are populated even when no configuration file is supplied.
// It returns a map describing which keys were generated so callers can log the event without exposing values.
func ApplyRuntimeDefaults(cfg *Config) (map[string]bool, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	generated := make(map[string]bool)

	if strings.TrimSpace(cfg.Auth.JWT.Secret) == "" {
		secret, err := crypto.GenerateToken(jwtSecretBytes)
		if err != nil {
			return nil, fmt.Errorf("generate jwt secret: %w", err)
		}
		cfg.Auth.JWT.Secret = secret
		generated["auth.jwt.secret"] = true
	}

	if strings.TrimSpace(cfg.Vault.EncryptionKey) == "" {
		secret, err := generateHexKey(vaultSecretBytes)
		if err != nil {
			return nil, fmt.Errorf("generate vault encryption key: %w", err)
		}
		cfg.Vault.EncryptionKey = secret
		generated["vault.encryption_key"] = true
	}

	return generated, nil
}

func generateHexKey(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be positive")
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
