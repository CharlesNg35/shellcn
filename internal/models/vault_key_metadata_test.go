package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVaultKeyMetadataBeforeSaveValidates(t *testing.T) {
	meta := &VaultKeyMetadata{}
	err := meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "key_id")

	meta.KeyID = "primary"
	err = meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kdf_algorithm")

	meta.KDFAlgorithm = "argon2id"
	err = meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "salt")

	meta.Salt = []byte("short")
	err = meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "salt")

	meta.Salt = bytesOfLength(32)
	err = meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kdf_parameters")

	params, _ := json.Marshal(map[string]any{"time": 2})
	meta.KDFParameters = params
	err = meta.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "material_hash")

	meta.MaterialHash = "hash"
	require.NoError(t, meta.BeforeSave(nil))
	require.False(t, meta.DerivedAt.IsZero())

	now := time.Now().UTC()
	meta.DerivedAt = now
	require.NoError(t, meta.BeforeSave(nil))
	require.WithinDuration(t, now, meta.DerivedAt, time.Second)
}

func bytesOfLength(n int) []byte {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte(i % 255)
	}
	return buf
}
