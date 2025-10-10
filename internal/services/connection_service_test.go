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

func TestConnectionServiceListVisible(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-123"},
		Username:  "viewer",
		Email:     "viewer@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	team := models.Team{
		BaseModel: models.BaseModel{ID: "team-1"},
		Name:      "Ops",
	}
	require.NoError(t, db.Create(&team).Error)
	require.NoError(t, db.Model(&user).Association("Teams").Append(&team))
	teamID := team.ID

	first := models.Connection{
		Name:        "Primary SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		Targets: []models.ConnectionTarget{
			{
				Host: "10.0.0.5",
				Port: 22,
			},
		},
	}
	require.NoError(t, db.Create(&first).Error)

	second := models.Connection{
		Name:        "Kubernetes control plane",
		ProtocolID:  "kubernetes",
		OwnerUserID: "user-other",
		Visibility: []models.ConnectionVisibility{
			{
				UserID:          &user.ID,
				PermissionScope: "view",
			},
		},
	}
	require.NoError(t, db.Create(&second).Error)

	third := models.Connection{
		Name:        "Team database",
		ProtocolID:  "postgres",
		OwnerUserID: "user-other",
		TeamID:      &teamID,
		Visibility: []models.ConnectionVisibility{
			{
				TeamID:          &team.ID,
				PermissionScope: "view",
			},
		},
	}
	require.NoError(t, db.Create(&third).Error)

	svc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.view":   true,
			"connection.manage": false,
		},
	})
	require.NoError(t, err)

	result, err := svc.ListVisible(context.Background(), ListConnectionsOptions{
		UserID:            user.ID,
		IncludeTargets:    true,
		IncludeVisibility: true,
	})
	require.NoError(t, err)
	require.Len(t, result.Connections, 3)

	names := make([]string, 0, len(result.Connections))
	for _, conn := range result.Connections {
		names = append(names, conn.Name)
	}
	require.Contains(t, names, "Primary SSH")
	require.Contains(t, names, "Kubernetes control plane")
	require.Contains(t, names, "Team database")

	teamScoped, err := svc.ListVisible(context.Background(), ListConnectionsOptions{
		UserID:            user.ID,
		TeamID:            team.ID,
		IncludeTargets:    false,
		IncludeVisibility: false,
	})
	require.NoError(t, err)
	require.Len(t, teamScoped.Connections, 1)
	require.Equal(t, "Team database", teamScoped.Connections[0].Name)
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

	svc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{"connection.view": true},
	})
	require.NoError(t, err)

	dto, err := svc.GetVisible(context.Background(), user.ID, private.ID, true, false)
	require.NoError(t, err)
	require.Equal(t, private.ID, dto.ID)
	require.Equal(t, "ssh", dto.ProtocolID)
	require.Equal(t, "critical", dto.Metadata["tag"])
}

func TestConnectionServiceCountByFolder(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-folders"},
		Username:  "folders",
		Email:     "folders@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	folder := models.ConnectionFolder{
		BaseModel:   models.BaseModel{ID: "folder-1"},
		Name:        "Production",
		OwnerUserID: user.ID,
	}
	require.NoError(t, db.Create(&folder).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Prod SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		FolderID:    &folder.ID,
	}).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Prod DB",
		ProtocolID:  "postgres",
		OwnerUserID: user.ID,
		FolderID:    &folder.ID,
	}).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Unassigned",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
	}).Error)

	svc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.view": true,
		},
	})
	require.NoError(t, err)

	counts, err := svc.CountByFolder(context.Background(), ListConnectionsOptions{UserID: user.ID})
	require.NoError(t, err)

	require.Equal(t, int64(2), counts["folder-1"])
	require.Equal(t, int64(1), counts["unassigned"])
}

func TestConnectionServiceCountByProtocol(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "user-protocol"},
		Username:  "proto",
		Email:     "proto@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	team := models.Team{
		BaseModel: models.BaseModel{ID: "team-proto"},
		Name:      "Platform",
	}
	require.NoError(t, db.Create(&team).Error)
	require.NoError(t, db.Model(&user).Association("Teams").Append(&team))

	teamID := team.ID
	require.NoError(t, db.Create(&models.Connection{
		Name:        "Team SSH",
		ProtocolID:  "ssh",
		OwnerUserID: user.ID,
		TeamID:      &teamID,
	}).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Team DB",
		ProtocolID:  "postgres",
		OwnerUserID: "other",
		TeamID:      &teamID,
		Visibility: []models.ConnectionVisibility{
			{
				TeamID:          &teamID,
				PermissionScope: "view",
			},
		},
	}).Error)

	require.NoError(t, db.Create(&models.Connection{
		Name:        "Personal RDP",
		ProtocolID:  "rdp",
		OwnerUserID: user.ID,
	}).Error)

	svc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.view": true,
		},
	})
	require.NoError(t, err)

	counts, err := svc.CountByProtocol(context.Background(), ListConnectionsOptions{UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts["rdp"])
	require.Equal(t, int64(1), counts["ssh"])
	require.Equal(t, int64(1), counts["postgres"])

	teamCounts, err := svc.CountByProtocol(context.Background(), ListConnectionsOptions{
		UserID: user.ID,
		TeamID: team.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), teamCounts["ssh"])
	require.Equal(t, int64(1), teamCounts["postgres"])
	require.Zero(t, teamCounts["rdp"])
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
