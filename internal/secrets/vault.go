package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// MasterKeySize is the required master-key length (AES-256).
const MasterKeySize = 32

const (
	vaultVersion = 1
	dekSize      = 32 // AES-256 data key
)

// Vault is the local AES-256-GCM SecretStore. Each Encrypt mints a random data
// key (DEK), encrypts the plaintext with it, then wraps the DEK with the master
// key (KEK). The self-describing blob layout is:
//
//	version(1) | kekNonce(12) | wrappedDEK(48) | dekNonce(12) | ciphertext(…)
//
// Rotating the master key means unwrapping every record's DEK with the old KEK
// and re-wrapping with the new one — the bulk ciphertext is untouched.
type Vault struct {
	kek cipher.AEAD
}

// NewVault builds a vault from a 32-byte master key.
func NewVault(masterKey []byte) (*Vault, error) {
	if len(masterKey) != MasterKeySize {
		return nil, fmt.Errorf("%w: want %d bytes, got %d", ErrMasterKey, MasterKeySize, len(masterKey))
	}
	aead, err := newGCM(masterKey)
	if err != nil {
		return nil, err
	}
	return &Vault{kek: aead}, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMasterKey, err)
	}
	return cipher.NewGCM(block)
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

// Encrypt seals plaintext into a self-describing envelope blob.
func (v *Vault) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	dek, err := randomBytes(dekSize)
	if err != nil {
		return nil, err
	}
	dekAEAD, err := newGCM(dek)
	if err != nil {
		return nil, err
	}

	kekNonce, err := randomBytes(v.kek.NonceSize())
	if err != nil {
		return nil, err
	}
	wrappedDEK := v.kek.Seal(nil, kekNonce, dek, nil)

	dekNonce, err := randomBytes(dekAEAD.NonceSize())
	if err != nil {
		return nil, err
	}
	ciphertext := dekAEAD.Seal(nil, dekNonce, plaintext, nil)

	blob := make([]byte, 0, 1+len(kekNonce)+len(wrappedDEK)+len(dekNonce)+len(ciphertext))
	blob = append(blob, vaultVersion)
	blob = append(blob, kekNonce...)
	blob = append(blob, wrappedDEK...)
	blob = append(blob, dekNonce...)
	blob = append(blob, ciphertext...)
	return blob, nil
}

// Decrypt opens an envelope blob produced by Encrypt.
func (v *Vault) Decrypt(_ context.Context, blob []byte) ([]byte, error) {
	kekNonceSize := v.kek.NonceSize()
	wrappedDEKSize := dekSize + v.kek.Overhead()
	// version + kekNonce + wrappedDEK + at least a dekNonce.
	minLen := 1 + kekNonceSize + wrappedDEKSize + 12
	if len(blob) < minLen || blob[0] != vaultVersion {
		return nil, ErrCiphertext
	}

	off := 1
	kekNonce := blob[off : off+kekNonceSize]
	off += kekNonceSize
	wrappedDEK := blob[off : off+wrappedDEKSize]
	off += wrappedDEKSize

	dek, err := v.kek.Open(nil, kekNonce, wrappedDEK, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: unwrap data key: %v", ErrCiphertext, err)
	}
	dekAEAD, err := newGCM(dek)
	if err != nil {
		return nil, err
	}

	dekNonceSize := dekAEAD.NonceSize()
	if len(blob) < off+dekNonceSize {
		return nil, ErrCiphertext
	}
	dekNonce := blob[off : off+dekNonceSize]
	off += dekNonceSize
	ciphertext := blob[off:]

	plaintext, err := dekAEAD.Open(nil, dekNonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCiphertext, err)
	}
	return plaintext, nil
}
