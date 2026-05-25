package secrets_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charlesng/shellcn/internal/secrets"
)

func newVault(t *testing.T) *secrets.Vault {
	t.Helper()
	key, err := secrets.GenerateMasterKey()
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	v, err := secrets.NewVault(key)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	return v
}

func TestVaultRoundTrip(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	plaintext := []byte("super-secret-private-key")

	blob, err := v.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Fatal("ciphertext contains the plaintext — not encrypted")
	}
	got, err := v.Decrypt(ctx, blob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch: got %q", got)
	}
}

func TestVaultUniqueCiphertext(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	a, _ := v.Encrypt(ctx, []byte("same"))
	b, _ := v.Encrypt(ctx, []byte("same"))
	if bytes.Equal(a, b) {
		t.Error("encrypting the same plaintext twice produced identical blobs (nonce/DEK reuse)")
	}
}

func TestVaultTamperDetected(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	blob, _ := v.Encrypt(ctx, []byte("x"))
	blob[len(blob)-1] ^= 0xFF // flip a ciphertext bit
	if _, err := v.Decrypt(ctx, blob); !errors.Is(err, secrets.ErrCiphertext) {
		t.Errorf("tampered blob should fail auth: got %v", err)
	}
}

func TestVaultWrongKeyCannotDecrypt(t *testing.T) {
	ctx := context.Background()
	v1 := newVault(t)
	v2 := newVault(t)
	blob, _ := v1.Encrypt(ctx, []byte("secret"))
	if _, err := v2.Decrypt(ctx, blob); err == nil {
		t.Error("a different master key must not decrypt the blob")
	}
}

func TestNewVaultRejectsBadKey(t *testing.T) {
	if _, err := secrets.NewVault([]byte("too short")); !errors.Is(err, secrets.ErrMasterKey) {
		t.Errorf("short key: want ErrMasterKey, got %v", err)
	}
}

func TestLoadMasterKeyFromEnv(t *testing.T) {
	key, _ := secrets.GenerateMasterKey()
	t.Setenv(secrets.EnvMasterKey, secrets.EncodeMasterKey(key))
	got, err := secrets.LoadMasterKey()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal(got, key) {
		t.Error("loaded key does not match")
	}
}

func TestLoadMasterKeyMissing(t *testing.T) {
	t.Setenv(secrets.EnvMasterKey, "")
	t.Setenv(secrets.EnvMasterKeyFile, "")
	if _, err := secrets.LoadMasterKey(); !errors.Is(err, secrets.ErrMasterKey) {
		t.Errorf("missing key: want ErrMasterKey, got %v", err)
	}
}

func TestRedactParams(t *testing.T) {
	params := map[string]string{"host": "10.0.0.1", "password": "hunter2"}
	got := secrets.RedactParams(params, map[string]bool{"password": true})
	if got["host"] != "10.0.0.1" {
		t.Errorf("non-secret redacted: %q", got["host"])
	}
	if got["password"] != secrets.Placeholder {
		t.Errorf("secret not redacted: %q", got["password"])
	}
	// Source map is not mutated.
	if params["password"] != "hunter2" {
		t.Error("RedactParams mutated its input")
	}
}

func TestState(t *testing.T) {
	if secrets.State(true) != "set" || secrets.State(false) != "not set" {
		t.Error("State strings wrong")
	}
}

func TestEncryptDecryptMap(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	in := map[string]string{"password": "correct-horse-battery-staple", "passphrase": "open-sesame-1234"}
	enc, err := secrets.EncryptMap(ctx, v, in)
	if err != nil {
		t.Fatalf("encrypt map: %v", err)
	}
	for k, blob := range enc {
		if strings.Contains(string(blob), in[k]) {
			t.Errorf("ciphertext for %q contains plaintext", k)
		}
	}
	dec, err := secrets.DecryptMap(ctx, v, enc)
	if err != nil {
		t.Fatalf("decrypt map: %v", err)
	}
	if dec["password"] != in["password"] || dec["passphrase"] != in["passphrase"] {
		t.Errorf("map round-trip mismatch: %+v", dec)
	}
}
