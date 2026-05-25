// Package secrets encrypts inline connection secrets and reusable credential
// material at rest behind a SecretStore interface, so the backend (local vault,
// KMS, …) is swappable without touching callers.
package secrets

import (
	"context"
	"errors"
)

// SecretStore encrypts and decrypts opaque blobs. Implementations keep the
// master key; callers never see it. The local vault uses envelope encryption
// (per-record data key wrapped by the master key); a KMS/OpenBao backend can
// drop in behind the same interface.
type SecretStore interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

var (
	// ErrMasterKey is returned when the master key is missing or the wrong size.
	ErrMasterKey = errors.New("secrets: invalid master key")
	// ErrCiphertext is returned when a blob is malformed or fails authentication.
	ErrCiphertext = errors.New("secrets: invalid ciphertext")
)
