package crypto

import (
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2Parameters controls the cost factors for Argon2id key derivation.
type Argon2Parameters struct {
	// Time is the number of iterations.
	Time uint32
	// Memory is the amount of memory (in kibibytes) to use.
	Memory uint32
	// Threads is the degree of parallelism.
	Threads uint8
	// KeyLength is the desired length of the derived key in bytes.
	KeyLength uint32
}

// DefaultArgon2Params returns the default Argon2id parameters used for vault key derivation.
func DefaultArgon2Params() Argon2Parameters {
	return Argon2Parameters{
		Time:      2,
		Memory:    64 * 1024, // 64 MiB
		Threads:   4,
		KeyLength: 32,
	}
}

// Validate ensures the parameters are suitable for Argon2id key derivation.
func (p Argon2Parameters) Validate() error {
	if p.Time == 0 {
		return fmt.Errorf("argon2: time cost must be greater than zero")
	}
	if p.Threads == 0 {
		return fmt.Errorf("argon2: parallelism must be greater than zero")
	}
	if p.Memory < 8*uint32(p.Threads) {
		return fmt.Errorf("argon2: memory cost must be at least 8 * threads")
	}
	if p.KeyLength == 0 {
		return fmt.Errorf("argon2: key length must be greater than zero")
	}
	switch p.KeyLength {
	case 16, 24, 32:
	default:
		return fmt.Errorf("argon2: key length must be 16, 24, or 32 bytes (got %d)", p.KeyLength)
	}
	return nil
}

// DeriveKeyArgon2id derives a key using the Argon2id KDF.
func DeriveKeyArgon2id(secret, salt []byte, params Argon2Parameters) ([]byte, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("argon2: secret is required")
	}
	if len(salt) < 16 {
		return nil, fmt.Errorf("argon2: salt must be at least 16 bytes (got %d)", len(salt))
	}
	if err := params.Validate(); err != nil {
		return nil, err
	}
	return argon2.IDKey(secret, salt, params.Time, params.Memory, params.Threads, params.KeyLength), nil
}
