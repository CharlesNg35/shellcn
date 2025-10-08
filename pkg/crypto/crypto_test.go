package crypto

import (
	"bytes"
	"testing"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}

	if !VerifyPassword(hash, "secret") {
		t.Fatal("expected password verification to succeed")
	}

	if VerifyPassword(hash, "incorrect") {
		t.Fatal("expected password verification to fail")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte{0x1}, 32)
	plaintext := []byte("sensitive data")

	encoded, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	decrypted, err := Decrypt(encoded, key)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("expected decrypted plaintext to match original, got %s", decrypted)
	}
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("token error: %v", err)
	}

	if len(token) == 0 {
		t.Fatal("expected token to be non-empty")
	}
}
