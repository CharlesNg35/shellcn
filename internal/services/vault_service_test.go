package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/vault"
)

func TestVaultServiceCreateAndGetIdentity(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, nil, crypto)
	require.NoError(t, err)

	viewer := ViewerContext{UserID: "user-1"}
	ctx := context.Background()

	created, err := svc.CreateIdentity(ctx, viewer, CreateIdentityInput{
		Name:        "Prod SSH",
		Scope:       models.IdentityScopeGlobal,
		Payload:     map[string]any{"username": "alice", "private_key": "---BEGIN---"},
		OwnerUserID: viewer.UserID,
		CreatedBy:   viewer.UserID,
	})
	require.NoError(t, err)
	require.Equal(t, "Prod SSH", created.Name)
	require.Equal(t, models.IdentityScopeGlobal, created.Scope)
	require.Nil(t, created.Payload)

	fetched, err := svc.GetIdentity(ctx, viewer, created.ID, true)
	require.NoError(t, err)
	require.Equal(t, created.ID, fetched.ID)
	require.Equal(t, 1, fetched.Version)
	require.NotNil(t, fetched.Payload)
	require.Equal(t, "alice", fetched.Payload["username"])
	require.Equal(t, 1, fetched.UsageCount)
	require.Equal(t, 0, fetched.ConnectionCount)

	updated, err := svc.UpdateIdentity(ctx, viewer, created.ID, UpdateIdentityInput{
		Description: stringPtr("rotated"),
	})
	require.NoError(t, err)
	require.Equal(t, "rotated", updated.Description)
	require.Equal(t, 0, updated.ConnectionCount)
}

func TestVaultServiceCreateShare(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, nil, crypto)
	require.NoError(t, err)

	viewer := ViewerContext{UserID: "owner-1"}
	ctx := context.Background()

	identity, err := svc.CreateIdentity(ctx, viewer, CreateIdentityInput{
		Name:        "Team Password",
		Scope:       models.IdentityScopeGlobal,
		Payload:     map[string]any{"password": "super-secret"},
		OwnerUserID: viewer.UserID,
		CreatedBy:   viewer.UserID,
	})
	require.NoError(t, err)

	expires := time.Now().Add(24 * time.Hour).UTC()
	share, err := svc.CreateShare(ctx, viewer, identity.ID, IdentityShareInput{
		PrincipalType: models.IdentitySharePrincipalUser,
		PrincipalID:   "user-2",
		Permission:    models.IdentitySharePermissionUse,
		ExpiresAt:     &expires,
		CreatedBy:     viewer.UserID,
	})
	require.NoError(t, err)
	require.Equal(t, models.IdentitySharePrincipalUser, share.PrincipalType)
	require.Equal(t, models.IdentitySharePermissionUse, share.Permission)

	fetched, err := svc.GetIdentity(ctx, viewer, identity.ID, false)
	require.NoError(t, err)
	require.Len(t, fetched.Shares, 1)
	require.Equal(t, share.ID, fetched.Shares[0].ID)
}

func TestVaultServiceListTemplates(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, nil, crypto)
	require.NoError(t, err)

	fields, err := json.Marshal([]map[string]any{{"key": "username", "type": "string"}})
	require.NoError(t, err)
	protocols, err := json.Marshal([]string{"ssh"})
	require.NoError(t, err)

	tpl := models.CredentialTemplate{
		DriverID:            "ssh",
		Version:             "1.0.0",
		DisplayName:         "SSH",
		Fields:              fields,
		CompatibleProtocols: protocols,
		Hash:                "hash",
	}
	require.NoError(t, db.Create(&tpl).Error)

	templates, err := svc.ListTemplates(context.Background())
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, "ssh", templates[0].DriverID)
	require.Equal(t, "1.0.0", templates[0].Version)
	require.Len(t, templates[0].Fields, 1)
}

func stringPtr(v string) *string {
	return &v
}
