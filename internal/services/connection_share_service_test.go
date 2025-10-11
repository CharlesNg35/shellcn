package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
)

func TestConnectionShareService_CreateListDelete(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	require.NoError(t, permissions.Sync(context.Background(), db))

	grantor := models.User{
		BaseModel: models.BaseModel{ID: "user-grantor"},
		Username:  "grantor",
		Email:     "grantor@example.com",
		Password:  "secret",
		IsRoot:    true,
	}
	require.NoError(t, db.Create(&grantor).Error)

	target := models.User{
		BaseModel: models.BaseModel{ID: "user-target"},
		Username:  "shared",
		Email:     "shared@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&target).Error)

	connection := models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-alpha"},
		Name:        "Alpha",
		ProtocolID:  "ssh",
		OwnerUserID: grantor.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	checker, err := permissions.NewChecker(db)
	require.NoError(t, err)

	svc, err := NewConnectionShareService(db, checker)
	require.NoError(t, err)

	expiry := time.Now().Add(2 * time.Hour).UTC()
	created, err := svc.CreateShare(context.Background(), grantor.ID, connection.ID, CreateShareInput{
		PrincipalType: "user",
		PrincipalID:   target.ID,
		PermissionIDs: []string{"connection.view", "connection.launch"},
		ExpiresAt:     &expiry,
		Metadata: map[string]any{
			"note": "temp access",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "user:"+target.ID, created.ShareID)
	require.ElementsMatch(t, []string{"connection.launch", "connection.view"}, created.PermissionScopes)
	require.NotNil(t, created.ExpiresAt)
	require.NotNil(t, created.GrantedBy)
	require.Equal(t, grantor.ID, created.GrantedBy.ID)
	require.Equal(t, target.ID, created.Principal.ID)
	require.Equal(t, "temp access", created.Metadata["note"])

	listed, err := svc.ListShares(context.Background(), grantor.ID, connection.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.ElementsMatch(t, []string{"connection.launch", "connection.view"}, listed[0].PermissionScopes)
	require.Equal(t, created.ShareID, listed[0].ShareID)
	require.Equal(t, "user", listed[0].Principal.Type)
	require.Equal(t, target.ID, listed[0].Principal.ID)
	require.NotNil(t, listed[0].GrantedBy)
	require.Equal(t, grantor.ID, listed[0].GrantedBy.ID)
	require.NotNil(t, listed[0].ExpiresAt)
	require.Equal(t, "temp access", listed[0].Metadata["note"])

	err = svc.DeleteShare(context.Background(), grantor.ID, connection.ID, listed[0].ShareID)
	require.NoError(t, err)

	afterDelete, err := svc.ListShares(context.Background(), grantor.ID, connection.ID)
	require.NoError(t, err)
	require.Empty(t, afterDelete)
}
