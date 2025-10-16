package services

import (
	context "context"
	testing "testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

type updatePermissionCheckerStub struct{}

func (s *updatePermissionCheckerStub) Check(context.Context, string, string) (bool, error) {
	return false, nil
}

func (s *updatePermissionCheckerStub) CheckResource(context.Context, string, string, string, string) (bool, error) {
	return false, nil
}

func TestConnectionServiceUpdate_AllowsOwner(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{BaseModel: models.BaseModel{ID: "user-1"}, Username: "owner", Email: "owner-update@example.com"}
	require.NoError(t, db.Create(&user).Error)

	connection := models.Connection{
		Name:        "Prod",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		Metadata:    datatypes.JSON([]byte(`{"icon":"terminal"}`)),
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, nil)
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), user.ID, connection.ID, UpdateConnectionInput{
		Name:        "Prod Updated",
		Description: "New description",
	})
	require.NoError(t, err)
	require.Equal(t, "Prod Updated", updated.Name)
	require.Equal(t, "New description", updated.Description)
}

func TestConnectionServiceUpdate_DeniesWithoutPermission(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	owner := models.User{BaseModel: models.BaseModel{ID: "owner"}, Username: "owner", Email: "owner2-update@example.com"}
	other := models.User{BaseModel: models.BaseModel{ID: "other"}, Username: "other", Email: "other-update@example.com"}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	connection := models.Connection{
		Name:        "Prod",
		ProtocolID:  "ssh",
		OwnerUserID: owner.ID,
	}
	require.NoError(t, db.Create(&connection).Error)

	svc, err := NewConnectionService(db, &updatePermissionCheckerStub{})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), other.ID, connection.ID, UpdateConnectionInput{
		Name: "Should fail",
	})
	require.ErrorIs(t, err, apperrors.ErrForbidden)
}
