package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/charlesng35/shellcn/pkg/crypto"
)

func TestNewCryptoDerivesKey(t *testing.T) {
	master, err := hex.DecodeString("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}

	vaultCrypto, err := NewCrypto(master)
	if err != nil {
		t.Fatalf("construct vault crypto: %v", err)
	}

	derivedKey := vaultCrypto.Key()
	if len(derivedKey) != 32 {
		t.Fatalf("expected derived key length 32, got %d", len(derivedKey))
	}

	expectedSalt := deriveSalt(master)
	if !bytes.Equal(expectedSalt, vaultCrypto.Salt()) {
		t.Fatalf("expected derived salt %x, got %x", expectedSalt, vaultCrypto.Salt())
	}
}

func TestCryptoEncryptDecrypt(t *testing.T) {
	master := []byte("super-secret-master-key")
	vaultCrypto, err := NewCrypto(master)
	if err != nil {
		t.Fatalf("construct vault crypto: %v", err)
	}

	plaintext := []byte("sensitive credential payload")
	ciphertext, err := vaultCrypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatal("expected ciphertext to be non-empty")
	}

	decrypted, err := vaultCrypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("expected decrypted plaintext to match original")
	}
}

func TestCryptoTamperingDetected(t *testing.T) {
	master := []byte("super-secret-master-key")
	vaultCrypto, err := NewCrypto(master)
	if err != nil {
		t.Fatalf("construct vault crypto: %v", err)
	}

	ciphertext, err := vaultCrypto.Encrypt([]byte("vault data"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("decode ciphertext: %v", err)
	}
	raw[len(raw)-1] ^= 0x01
	ciphertext = base64.StdEncoding.EncodeToString(raw)

	if _, err := vaultCrypto.Decrypt(ciphertext); err == nil {
		t.Fatal("expected decrypt to fail for tampered ciphertext")
	}
}

func TestNewCryptoWithCustomSalt(t *testing.T) {
	master := []byte("master-secret")
	customSalt := bytes.Repeat([]byte{0x5A}, 32)

	vaultCrypto, err := NewCrypto(master, WithSalt(customSalt))
	if err != nil {
		t.Fatalf("construct vault crypto: %v", err)
	}

	if !bytes.Equal(customSalt, vaultCrypto.Salt()) {
		t.Fatalf("expected salt override to be applied")
	}
}

func TestNewCryptoValidatesArgs(t *testing.T) {
	_, err := NewCrypto(nil)
	if err == nil {
		t.Fatal("expected error when master key is empty")
	}

	master := []byte("master")
	if _, err := NewCrypto(master, WithSalt([]byte("short"))); err == nil {
		t.Fatal("expected error for short salt")
	}

	params := crypto.DefaultArgon2Params()
	params.KeyLength = 20
	if _, err := NewCrypto(master, WithArgon2Parameters(params)); err == nil {
		t.Fatal("expected error for invalid argon2 parameters")
	}
}
