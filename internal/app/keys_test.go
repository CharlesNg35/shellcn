package app

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestDecodeKeyHex(t *testing.T) {
	// 32 bytes = 64 hex characters
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	decoded, err := DecodeKey(hexKey)
	if err != nil {
		t.Fatalf("DecodeKey failed: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(decoded))
	}

	// Verify it decodes correctly
	expected, _ := hex.DecodeString(hexKey)
	if string(decoded) != string(expected) {
		t.Fatal("decoded bytes don't match expected hex decoding")
	}
}

func TestDecodeKeyBase64(t *testing.T) {
	// Create a 32-byte key and encode it as base64
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i)
	}
	base64Key := base64.StdEncoding.EncodeToString(rawKey)

	decoded, err := DecodeKey(base64Key)
	if err != nil {
		t.Fatalf("DecodeKey failed: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(decoded))
	}
	if string(decoded) != string(rawKey) {
		t.Fatal("decoded bytes don't match expected base64 decoding")
	}
}

func TestDecodeKeyRawBytes(t *testing.T) {
	// If it's not valid hex or base64, treat as raw bytes
	rawKey := "this-is-a-raw-32-byte-key!!!"
	decoded, err := DecodeKey(rawKey)
	if err != nil {
		t.Fatalf("DecodeKey failed: %v", err)
	}
	if string(decoded) != rawKey {
		t.Fatal("decoded bytes don't match raw input")
	}
}

func TestDecodeKeyEmpty(t *testing.T) {
	_, err := DecodeKey("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestKeyByteLengthHex(t *testing.T) {
	// 32 bytes = 64 hex characters
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	length, err := KeyByteLength(hexKey)
	if err != nil {
		t.Fatalf("KeyByteLength failed: %v", err)
	}
	if length != 32 {
		t.Fatalf("expected 32 bytes, got %d", length)
	}
}

func TestKeyByteLengthBase64(t *testing.T) {
	// Create a 32-byte key and encode it as base64
	rawKey := make([]byte, 32)
	base64Key := base64.StdEncoding.EncodeToString(rawKey)

	length, err := KeyByteLength(base64Key)
	if err != nil {
		t.Fatalf("KeyByteLength failed: %v", err)
	}
	if length != 32 {
		t.Fatalf("expected 32 bytes, got %d", length)
	}
}

func TestKeyByteLengthRawBytes(t *testing.T) {
	rawKey := "this-is-a-raw-key"
	length, err := KeyByteLength(rawKey)
	if err != nil {
		t.Fatalf("KeyByteLength failed: %v", err)
	}
	if length != len(rawKey) {
		t.Fatalf("expected %d bytes, got %d", len(rawKey), length)
	}
}

func TestKeyByteLengthEmpty(t *testing.T) {
	length, err := KeyByteLength("")
	if err != nil {
		t.Fatalf("KeyByteLength failed: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected 0 bytes for empty string, got %d", length)
	}
}

func TestKeyByteLengthWhitespace(t *testing.T) {
	length, err := KeyByteLength("   ")
	if err != nil {
		t.Fatalf("KeyByteLength failed: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected 0 bytes for whitespace, got %d", length)
	}
}
