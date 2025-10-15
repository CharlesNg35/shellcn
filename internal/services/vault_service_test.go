package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
	"github.com/charlesng35/shellcn/internal/vault"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
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

func TestVaultServiceLoadIdentitySecret(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, nil, crypto)
	require.NoError(t, err)

	owner := ViewerContext{UserID: "owner-1"}
	recipient := ViewerContext{UserID: "user-2"}

	ctx := context.Background()

	identity, err := svc.CreateIdentity(ctx, owner, CreateIdentityInput{
		Name:        "Deploy Key",
		Scope:       models.IdentityScopeGlobal,
		Payload:     map[string]any{"username": "deploy", "private_key": "PEM"},
		OwnerUserID: owner.UserID,
		CreatedBy:   owner.UserID,
	})
	require.NoError(t, err)

	_, err = svc.CreateShare(ctx, owner, identity.ID, IdentityShareInput{
		PrincipalType: models.IdentitySharePrincipalUser,
		PrincipalID:   recipient.UserID,
		Permission:    models.IdentitySharePermissionUse,
		CreatedBy:     owner.UserID,
	})
	require.NoError(t, err)

	secret, err := svc.LoadIdentitySecret(ctx, recipient, identity.ID)
	require.NoError(t, err)
	require.Equal(t, "deploy", secret["username"])
	require.Equal(t, "PEM", secret["private_key"])
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

func TestVaultServiceCreateIdentityRequiresPermission(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, checker, crypto)
	require.NoError(t, err)

	user := models.User{
		Username: "noperm",
		Email:    "noperm@example.com",
		Password: "hashed-password",
	}
	require.NoError(t, db.Create(&user).Error)

	viewer := ViewerContext{UserID: user.ID}
	_, err = svc.CreateIdentity(context.Background(), viewer, CreateIdentityInput{
		Name:        "Restricted credential",
		Scope:       models.IdentityScopeGlobal,
		Payload:     map[string]any{"secret": "value"},
		OwnerUserID: user.ID,
		CreatedBy:   user.ID,
	})
	require.ErrorIs(t, err, apperrors.ErrForbidden)
}

func TestVaultServiceShareFlows(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithSeedData())

	auditSvc, err := NewAuditService(db)
	require.NoError(t, err)

	crypto, err := vault.NewCrypto([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	svc, err := NewVaultService(db, auditSvc, checker, crypto)
	require.NoError(t, err)

	owner := models.User{
		Username: "vault-owner",
		Email:    "owner@example.com",
		Password: "hashed-password",
	}
	require.NoError(t, db.Create(&owner).Error)

	var adminRole models.Role
	require.NoError(t, db.First(&adminRole, "id = ?", "admin").Error)
	require.NoError(t, db.Model(&owner).Association("Roles").Append(&adminRole))

	recip := models.User{
		Username: "recipient",
		Email:    "recipient@example.com",
		Password: "hashed-password",
	}
	require.NoError(t, db.Create(&recip).Error)

	var userRole models.Role
	require.NoError(t, db.First(&userRole, "id = ?", "user").Error)
	require.NoError(t, db.Model(&recip).Association("Roles").Append(&userRole))

	ctx := context.Background()

	ownerViewer, err := svc.ResolveViewer(ctx, owner.ID, false)
	require.NoError(t, err)

	recipViewer, err := svc.ResolveViewer(ctx, recip.ID, false)
	require.NoError(t, err)

	identity, err := svc.CreateIdentity(ctx, ownerViewer, CreateIdentityInput{
		Name:        "Production API Token",
		Scope:       models.IdentityScopeGlobal,
		Payload:     map[string]any{"token": "super-secret-token"},
		OwnerUserID: owner.ID,
		CreatedBy:   owner.ID,
	})
	require.NoError(t, err)

	_, err = svc.CreateShare(ctx, ownerViewer, identity.ID, IdentityShareInput{
		PrincipalType: models.IdentitySharePrincipalUser,
		PrincipalID:   recip.ID,
		Permission:    models.IdentitySharePermissionUse,
		CreatedBy:     owner.ID,
	})
	require.NoError(t, err)

	_, err = svc.AuthorizeIdentityUse(ctx, recipViewer, identity.ID)
	require.NoError(t, err)

	snapshot, err := svc.GetIdentity(ctx, recipViewer, identity.ID, false)
	require.NoError(t, err)
	require.Equal(t, identity.ID, snapshot.ID)

	_, err = svc.GetIdentity(ctx, recipViewer, identity.ID, true)
	require.ErrorIs(t, err, apperrors.ErrForbidden)

	_, err = svc.CreateShare(ctx, ownerViewer, identity.ID, IdentityShareInput{
		PrincipalType: models.IdentitySharePrincipalUser,
		PrincipalID:   recip.ID,
		Permission:    models.IdentitySharePermissionEdit,
		CreatedBy:     owner.ID,
	})
	require.NoError(t, err)

	withPayload, err := svc.GetIdentity(ctx, recipViewer, identity.ID, true)
	require.NoError(t, err)
	require.Equal(t, "super-secret-token", withPayload.Payload["token"])

	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("action = ?", "vault.identity.shared").
		Count(&auditCount).Error)
	require.GreaterOrEqual(t, auditCount, int64(2))
}
