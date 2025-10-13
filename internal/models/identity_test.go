package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityBeforeSaveGlobalScope(t *testing.T) {
	identity := &Identity{
		Name:             "  Root Key  ",
		Scope:            IdentityScopeGlobal,
		OwnerUserID:      "user-123",
		EncryptedPayload: "ciphertext",
	}

	require.NoError(t, identity.BeforeSave(nil))
	require.Equal(t, "Root Key", identity.Name)
	require.Equal(t, IdentityScopeGlobal, identity.Scope)
	require.Nil(t, identity.TeamID)
	require.Nil(t, identity.ConnectionID)
	require.Equal(t, 1, identity.Version)
}

func TestIdentityBeforeSaveTeamScopeRequiresTeamID(t *testing.T) {
	identity := &Identity{
		Name:             "Team Secret",
		Scope:            IdentityScopeTeam,
		OwnerUserID:      "user-123",
		EncryptedPayload: "payload",
	}

	err := identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "team_id")

	teamID := "team-1"
	identity.TeamID = &teamID
	require.NoError(t, identity.BeforeSave(nil))
	require.Nil(t, identity.ConnectionID)
}

func TestIdentityBeforeSaveConnectionScope(t *testing.T) {
	identity := &Identity{
		Name:             "Connection Secret",
		Scope:            IdentityScopeConnection,
		OwnerUserID:      "user-123",
		EncryptedPayload: "cipher",
	}

	teamID := "team-1"
	identity.TeamID = &teamID
	err := identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "team_id")

	identity.TeamID = nil
	require.NoError(t, identity.BeforeSave(nil))
}

func TestIdentityBeforeSaveValidatesFields(t *testing.T) {
	identity := &Identity{}
	err := identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name")

	identity.Name = "vault"
	err = identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "owner_user_id")

	identity.OwnerUserID = "owner"
	err = identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "scope")

	identity.Scope = IdentityScopeGlobal
	err = identity.BeforeSave(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "encrypted_payload")
}
