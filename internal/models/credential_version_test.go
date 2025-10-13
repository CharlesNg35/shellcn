package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentialVersionBeforeSaveValidates(t *testing.T) {
	version := &CredentialVersion{}
	err := version.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity_id")

	version.IdentityID = "identity-1"
	err = version.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "version")

	version.Version = 1
	err = version.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "encrypted_payload")

	version.EncryptedPayload = "cipher"
	err = version.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "created_by")

	version.CreatedBy = "admin"
	require.NoError(t, version.BeforeSave(nil))
}
