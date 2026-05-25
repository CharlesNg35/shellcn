package secrets

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Environment variables the master key is loaded from.
const (
	EnvMasterKey     = "SHELLCN_MASTER_KEY"      // base64-encoded 32-byte key
	EnvMasterKeyFile = "SHELLCN_MASTER_KEY_FILE" // path to a file holding the key
)

// LoadMasterKey reads the AES-256 master key from the environment: EnvMasterKey
// (base64) takes precedence, else EnvMasterKeyFile (base64 or raw 32 bytes).
// KMS/OpenBao is a later drop-in behind the same Vault construction.
func LoadMasterKey() ([]byte, error) {
	if raw := strings.TrimSpace(os.Getenv(EnvMasterKey)); raw != "" {
		return decodeKey([]byte(raw))
	}
	if path := os.Getenv(EnvMasterKeyFile); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%w: read key file: %v", ErrMasterKey, err)
		}
		return decodeKey(data)
	}
	return nil, fmt.Errorf("%w: set %s or %s", ErrMasterKey, EnvMasterKey, EnvMasterKeyFile)
}

// decodeKey accepts a raw 32-byte key or its base64 (std or url) encoding.
func decodeKey(data []byte) ([]byte, error) {
	data = bytes.TrimSpace(data)
	if len(data) == MasterKeySize {
		return data, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(string(data)); err == nil && len(dec) == MasterKeySize {
			return dec, nil
		}
	}
	return nil, fmt.Errorf("%w: key must be %d raw bytes or base64 thereof", ErrMasterKey, MasterKeySize)
}

// GenerateMasterKey returns a fresh random 32-byte key (for dev / tooling).
func GenerateMasterKey() ([]byte, error) {
	return randomBytes(MasterKeySize)
}

// EncodeMasterKey renders a key as base64 for storage in env/file.
func EncodeMasterKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}
