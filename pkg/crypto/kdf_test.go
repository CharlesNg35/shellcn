package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveKeyArgon2idDeterministic(t *testing.T) {
	params := DefaultArgon2Params()
	secret := []byte("super-secret-passphrase")
	salt := bytes.Repeat([]byte{0xA5}, 16)

	key1, err := DeriveKeyArgon2id(secret, salt, params)
	if err != nil {
		t.Fatalf("derive key (first): %v", err)
	}
	key2, err := DeriveKeyArgon2id(secret, salt, params)
	if err != nil {
		t.Fatalf("derive key (second): %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Fatalf("expected deterministic key derivation; keys differ")
	}
	if len(key1) != int(params.KeyLength) {
		t.Fatalf("expected key length %d, got %d", params.KeyLength, len(key1))
	}
}

func TestDeriveKeyArgon2idDifferentSalts(t *testing.T) {
	params := DefaultArgon2Params()
	secret := []byte("super-secret-passphrase")
	saltA := bytes.Repeat([]byte{0x01}, 16)
	saltB := bytes.Repeat([]byte{0x02}, 16)

	keyA, err := DeriveKeyArgon2id(secret, saltA, params)
	if err != nil {
		t.Fatalf("derive key (A): %v", err)
	}
	keyB, err := DeriveKeyArgon2id(secret, saltB, params)
	if err != nil {
		t.Fatalf("derive key (B): %v", err)
	}

	if bytes.Equal(keyA, keyB) {
		t.Fatalf("expected different keys for different salts")
	}
}

func TestDeriveKeyArgon2idValidatesInput(t *testing.T) {
	params := DefaultArgon2Params()
	secret := []byte("secret")
	shortSalt := []byte("short")

	if _, err := DeriveKeyArgon2id(nil, bytes.Repeat([]byte{0x01}, 16), params); err == nil {
		t.Fatal("expected error when secret is empty")
	}

	if _, err := DeriveKeyArgon2id(secret, shortSalt, params); err == nil {
		t.Fatal("expected error when salt is too short")
	}

	badParams := params
	badParams.KeyLength = 20
	if _, err := DeriveKeyArgon2id(secret, bytes.Repeat([]byte{0x02}, 16), badParams); err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestArgon2ParametersValidate(t *testing.T) {
	cases := []struct {
		name   string
		params Argon2Parameters
		valid  bool
	}{
		{"default", DefaultArgon2Params(), true},
		{"zero time", Argon2Parameters{Time: 0, Memory: 64 * 1024, Threads: 4, KeyLength: 32}, false},
		{"zero threads", Argon2Parameters{Time: 2, Memory: 64 * 1024, Threads: 0, KeyLength: 32}, false},
		{"low memory", Argon2Parameters{Time: 2, Memory: 16, Threads: 4, KeyLength: 32}, false},
		{"zero key length", Argon2Parameters{Time: 2, Memory: 64 * 1024, Threads: 4, KeyLength: 0}, false},
		{"invalid key length", Argon2Parameters{Time: 2, Memory: 64 * 1024, Threads: 4, KeyLength: 48}, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.valid && err != nil {
				t.Fatalf("expected params to be valid: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected validation error for params")
			}
		})
	}
}
