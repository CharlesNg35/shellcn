package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestConnectionFolderServiceListTree(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "root-user"},
		Username:  "root",
		Email:     "root@example.com",
		Password:  "secret",
		IsRoot:    true,
	}
	require.NoError(t, db.Create(&user).Error)

	folder := models.ConnectionFolder{
		Name:        "Production",
		Slug:        "production",
		OwnerUserID: user.ID,
	}
	require.NoError(t, db.Create(&folder).Error)

	connection := models.Connection{
		Name:        "Prod SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		FolderID:    &folder.ID,
		Metadata:    datatypes.JSON("{}"),
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, &fakePermissionChecker{grants: map[string]bool{
		"connection.view":          true,
		"connection.folder.manage": true,
	}})
	require.NoError(t, err)

	folderSvc, err := NewConnectionFolderService(db, &fakePermissionChecker{grants: map[string]bool{
		"connection.folder.manage": true,
		"connection.folder.view":   true,
		"connection.view":          true,
	}}, svc)
	require.NoError(t, err)

	tree, err := folderSvc.ListTree(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, tree)
	require.Equal(t, int64(1), tree[0].ConnectionCount)
}
