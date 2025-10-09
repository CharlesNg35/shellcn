package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/models"
)

type fakePermissionChecker struct {
	grants map[string]bool
	err    error
}

func (f *fakePermissionChecker) Check(_ context.Context, _ string, permissionID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.grants[permissionID], nil
}

func TestConnectionServiceListVisible(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	orgID := "org-123"
	user := models.User{
		BaseModel:      models.BaseModel{ID: "user-123"},
		Username:       "viewer",
		Email:          "viewer@example.com",
		Password:       "secret",
		OrganizationID: &orgID,
	}
	require.NoError(t, db.Create(&user).Error)

	team := models.Team{
		BaseModel:      models.BaseModel{ID: "team-1"},
		Name:           "Ops",
		OrganizationID: orgID,
	}
	require.NoError(t, db.Create(&team).Error)
	require.NoError(t, db.Model(&user).Association("Teams").Append(&team))

	first := models.Connection{
		Name:        "Primary SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		Targets: []models.ConnectionTarget{
			{
				Host: "10.0.0.5",
				Port: ptrInt(22),
			},
		},
	}
	require.NoError(t, db.Create(&first).Error)

	second := models.Connection{
		Name:           "Kubernetes control plane",
		ProtocolID:     "kubernetes",
		OrganizationID: &orgID,
		OwnerUserID:    "user-other",
		Visibility: []models.ConnectionVisibility{
			{
				OrganizationID:  &orgID,
				PermissionScope: "view",
			},
		},
	}
	require.NoError(t, db.Create(&second).Error)

	third := models.Connection{
		Name:        "Team database",
		ProtocolID:  "postgres",
		OwnerUserID: "user-other",
		Visibility: []models.ConnectionVisibility{
			{
				TeamID:          &team.ID,
				PermissionScope: "view",
			},
		},
	}
	require.NoError(t, db.Create(&third).Error)

	checker := &fakePermissionChecker{
		grants: map[string]bool{
			"connection.view":   true,
			"connection.manage": false,
		},
	}

	svc, err := NewConnectionService(db, checker)
	require.NoError(t, err)

	result, err := svc.ListVisible(context.Background(), ListConnectionsOptions{
		UserID:            user.ID,
		IncludeTargets:    true,
		IncludeVisibility: true,
	})
	require.NoError(t, err)
	require.Len(t, result.Connections, 3)

	names := []string{
		result.Connections[0].Name,
		result.Connections[1].Name,
		result.Connections[2].Name,
	}
	require.Contains(t, names, "Primary SSH")
	require.Contains(t, names, "Kubernetes control plane")
	require.Contains(t, names, "Team database")
}

func TestConnectionServiceGetVisibleRespectsPermissions(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-900"},
		Username:  "limited",
		Email:     "limited@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	private := models.Connection{
		BaseModel:   models.BaseModel{ID: "conn-private"},
		Name:        "Private Jump Host",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		Metadata:    datatypes.JSON(mustJSON(t, map[string]any{"tag": "critical"})),
	}
	require.NoError(t, db.Create(&private).Error)

	svc, err := NewConnectionService(db, &fakePermissionChecker{
		grants: map[string]bool{"connection.view": true},
	})
	require.NoError(t, err)

	dto, err := svc.GetVisible(context.Background(), user.ID, private.ID, true, false)
	require.NoError(t, err)
	require.Equal(t, private.ID, dto.ID)
	require.Equal(t, "ssh", dto.ProtocolID)
	require.Equal(t, "critical", dto.Metadata["tag"])
}

func ptrInt(v int) *int {
	return &v
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
