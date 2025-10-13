package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityShareBeforeSaveValidates(t *testing.T) {
	share := &IdentityShare{}
	err := share.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity_id")

	share.IdentityID = "identity-1"
	err = share.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "principal_id")

	share.PrincipalID = "user-1"
	share.PrincipalType = IdentitySharePrincipalUser
	share.Permission = IdentitySharePermissionUse
	share.CreatedBy = "admin"
	share.UpdatedBy = "admin"
	share.GrantedBy = "admin"

	require.NoError(t, share.BeforeSave(nil))

	share.Permission = IdentitySharePermission("unknown")
	err = share.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid permission")
}
