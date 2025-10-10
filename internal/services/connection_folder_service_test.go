package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

func TestConnectionFolderServiceLifecycle(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-folders"},
		Username:  "folders",
		Email:     "folders@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	connectionSvc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.view": true,
		},
	})
	require.NoError(t, err)

	folderSvc, err := NewConnectionFolderService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.folder.view":   true,
			"connection.folder.manage": true,
			"connection.view":          true,
		},
	}, connectionSvc)
	require.NoError(t, err)

	root, err := folderSvc.Create(context.Background(), user.ID, ConnectionFolderInput{
		Name:     "Production",
		Metadata: map[string]any{"env": "prod"},
	})
	require.NoError(t, err)
	require.Equal(t, "production", root.Slug)

	child, err := folderSvc.Create(context.Background(), user.ID, ConnectionFolderInput{
		Name:     "Web Tier",
		ParentID: &root.ID,
	})
	require.NoError(t, err)
	require.Equal(t, root.ID, *child.ParentID)

	updated, err := folderSvc.Update(context.Background(), user.ID, child.ID, ConnectionFolderInput{
		Description: "Handles HTTP ingress",
		Color:       "#ff00aa",
	})
	require.NoError(t, err)
	require.Equal(t, "Handles HTTP ingress", updated.Description)
	require.Equal(t, "#ff00aa", updated.Color)

	// Seed connections for tree counts.
	require.NoError(t, db.Create(&models.Connection{
		Name:        "Ingress SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		FolderID:    &child.ID,
		Metadata:    datatypes.JSON("{}"),
	}).Error)
	require.NoError(t, db.Create(&models.Connection{
		Name:        "Unassigned Conn",
		ProtocolID:  "postgres",
		OwnerUserID: user.ID,
	}).Error)

	tree, err := folderSvc.ListTree(context.Background(), user.ID, nil)
	require.NoError(t, err)
	require.NotEmpty(t, tree)

	total := int64(0)
	for _, node := range tree {
		total += node.ConnectionCount
	}
	require.Equal(t, int64(2), total)

	personal := "personal"
	personalTree, err := folderSvc.ListTree(context.Background(), user.ID, &personal)
	require.NoError(t, err)
	require.Len(t, personalTree, 2)
	require.Equal(t, "unassigned", personalTree[0].Folder.ID)
	require.Equal(t, int64(1), personalTree[0].ConnectionCount)
	require.Equal(t, root.ID, personalTree[1].Folder.ID)
	require.Equal(t, int64(1), personalTree[1].ConnectionCount)

	require.NoError(t, folderSvc.Delete(context.Background(), user.ID, child.ID))

	remainingTree, err := folderSvc.ListTree(context.Background(), user.ID, nil)
	require.NoError(t, err)
	require.NotEmpty(t, remainingTree)
}
