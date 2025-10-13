package vault

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/charlesng35/shellcn/pkg/crypto"
)

const defaultSaltLength = 16

// Crypto provides encryption helpers for vault secrets.
type Crypto struct {
	key    []byte
	salt   []byte
	params crypto.Argon2Parameters
}

type cryptoConfig struct {
	params crypto.Argon2Parameters
	salt   []byte
}

// Option configures the vault crypto helper.
type Option func(*cryptoConfig)

// WithSalt overrides the salt used for Argon2 key derivation.
func WithSalt(salt []byte) Option {
	cp := make([]byte, len(salt))
	copy(cp, salt)
	return func(cfg *cryptoConfig) {
		cfg.salt = cp
	}
}

// WithArgon2Parameters overrides the Argon2 parameters used during key derivation.
func WithArgon2Parameters(params crypto.Argon2Parameters) Option {
	return func(cfg *cryptoConfig) {
		cfg.params = params
	}
}

// NewCrypto derives an AES key from the provided master key using Argon2id.
func NewCrypto(masterKey []byte, opts ...Option) (*Crypto, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("vault crypto: master key is required")
	}

	cfg := cryptoConfig{
		params: crypto.DefaultArgon2Params(),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if len(cfg.salt) == 0 {
		cfg.salt = deriveSalt(masterKey)
	} else if len(cfg.salt) < defaultSaltLength {
		return nil, fmt.Errorf("vault crypto: salt must be at least %d bytes (got %d)", defaultSaltLength, len(cfg.salt))
	}

	derived, err := crypto.DeriveKeyArgon2id(masterKey, cfg.salt, cfg.params)
	if err != nil {
		return nil, fmt.Errorf("vault crypto: derive key: %w", err)
	}

	return &Crypto{
		key:    derived,
		salt:   append([]byte(nil), cfg.salt...),
		params: cfg.params,
	}, nil
}

// Encrypt encrypts plaintext bytes using the derived AES-256-GCM key.
func (c *Crypto) Encrypt(plaintext []byte) (string, error) {
	if len(c.key) == 0 {
		return "", errors.New("vault crypto: key is not initialised")
	}
	return crypto.Encrypt(plaintext, c.key)
}

// Decrypt decrypts an encrypted payload using the derived AES-256-GCM key.
func (c *Crypto) Decrypt(ciphertext string) ([]byte, error) {
	if len(c.key) == 0 {
		return nil, errors.New("vault crypto: key is not initialised")
	}
	return crypto.Decrypt(ciphertext, c.key)
}

// Key returns a copy of the derived key bytes.
func (c *Crypto) Key() []byte {
	return append([]byte(nil), c.key...)
}

// Salt returns a copy of the salt used during derivation.
func (c *Crypto) Salt() []byte {
	return append([]byte(nil), c.salt...)
}

// Parameters returns the Argon2 parameters used during derivation.
func (c *Crypto) Parameters() crypto.Argon2Parameters {
	return c.params
}

func deriveSalt(masterKey []byte) []byte {
	sum := sha256.Sum256(masterKey)
	return sum[:defaultSaltLength]
}
