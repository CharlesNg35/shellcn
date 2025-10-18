package services

import (
	context "context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

type deletePermissionCheckerStub struct {
	allow bool
}

func (s *deletePermissionCheckerStub) Check(ctx context.Context, userID, permissionID string) (bool, error) {
	if s.allow && (permissionID == "connection.delete" || permissionID == "connection.manage") {
		return true, nil
	}
	return false, nil
}

func (s *deletePermissionCheckerStub) CheckResource(ctx context.Context, userID, resourceType, resourceID, permissionID string) (bool, error) {
	if s.allow && (permissionID == "connection.delete" || permissionID == "connection.manage") {
		return true, nil
	}
	return false, nil
}

func TestConnectionServiceDelete_AllowsOwner(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	owner := models.User{BaseModel: models.BaseModel{ID: "owner-del"}, Username: "owner", Email: "owner-delete@example.com"}
	require.NoError(t, db.Create(&owner).Error)

	connection := models.Connection{
		Name:        "Prod",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, nil)
	require.NoError(t, err)

	err = svc.Delete(context.Background(), owner.ID, connection.ID)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Connection{}).Where("id = ?", connection.ID).Count(&count).Error)
	require.Zero(t, count)
}

func TestConnectionServiceDelete_DeniesWithoutPermission(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	owner := models.User{BaseModel: models.BaseModel{ID: "owner-del-deny"}, Username: "owner", Email: "owner-deny-delete@example.com"}
	other := models.User{BaseModel: models.BaseModel{ID: "other-del-deny"}, Username: "other", Email: "other-deny-delete@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	connection := models.Connection{
		Name:        "Prod",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, &deletePermissionCheckerStub{allow: false})
	require.NoError(t, err)

	err = svc.Delete(context.Background(), other.ID, connection.ID)
	require.ErrorIs(t, err, apperrors.ErrForbidden)

	var count int64
	require.NoError(t, db.Model(&models.Connection{}).Where("id = ?", connection.ID).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestConnectionServiceDelete_AllowsWithPermission(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	owner := models.User{BaseModel: models.BaseModel{ID: "owner-del-allow"}, Username: "owner", Email: "owner-allow-delete@example.com"}
	other := models.User{BaseModel: models.BaseModel{ID: "other-del-allow"}, Username: "other", Email: "other-allow-delete@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	connection := models.Connection{
		Name:        "Prod",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, &deletePermissionCheckerStub{allow: true})
	require.NoError(t, err)

	err = svc.Delete(context.Background(), other.ID, connection.ID)
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.Connection{}).Where("id = ?", connection.ID).Count(&count).Error)
	require.Zero(t, count)
}
