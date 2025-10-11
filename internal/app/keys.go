package app

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// DecodeKey decodes a key from hex or base64 encoding to raw bytes.
// It tries hex first (since runtime defaults use hex), then base64 variants.
// If all decoding attempts fail, it treats the input as raw bytes.
func DecodeKey(value string) ([]byte, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil, fmt.Errorf("key value is empty")
	}

	// Try hex first (runtime defaults use hex for vault key)
	if len(v)%2 == 0 {
		if decoded, err := hex.DecodeString(v); err == nil {
			return decoded, nil
		}
	}

	// Support both standard and raw base64 encodings
	if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		return decoded, nil
	}

	// Fallback to treating as raw bytes
	return []byte(v), nil
}

// KeyByteLength returns the decoded byte length of a key string.
// It supports hex, base64, and raw string encodings.
func KeyByteLength(value string) (int, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, nil
	}

	// Try hex first (runtime defaults use hex for vault key)
	if len(v)%2 == 0 {
		if decoded, err := hex.DecodeString(v); err == nil {
			return len(decoded), nil
		}
	}

	// Support both standard and raw base64 encodings
	if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
		return len(decoded), nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(v); err == nil {
		return len(decoded), nil
	}

	return len(v), nil
}
