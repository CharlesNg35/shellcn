package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
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
	}
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Create(&models.ResourcePermission{
		ResourceID:    second.ID,
		ResourceType:  "connection",
		PrincipalType: "user",
		PrincipalID:   user.ID,
		PermissionID:  "connection.view",
	}).Error)

	third := models.Connection{
		Name:        "Team database",
		ProtocolID:  "postgres",
		OwnerUserID: "user-other",
		TeamID:      &teamID,
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
		UserID:         user.ID,
		IncludeTargets: true,
		IncludeGrants:  true,
	})
	require.NoError(t, err)
	require.Len(t, result.Connections, 3)

	names := make([]string, 0, len(result.Connections))
	var sharedShares []ConnectionShareDTO
	var shareSummary *ConnectionShareSummary
	for _, conn := range result.Connections {
		names = append(names, conn.Name)
		if conn.ID == second.ID {
			sharedShares = conn.Shares
			shareSummary = conn.ShareSummary
		}
	}
	require.Contains(t, names, "Primary SSH")
	require.Contains(t, names, "Kubernetes control plane")
	require.Contains(t, names, "Team database")
	require.Len(t, sharedShares, 1)
	require.Equal(t, "user:"+user.ID, sharedShares[0].ShareID)
	require.ElementsMatch(t, []string{"connection.view"}, sharedShares[0].PermissionScopes)
	require.NotNil(t, shareSummary)
	require.True(t, shareSummary.Shared)
	require.Len(t, shareSummary.Entries, 1)
	require.Equal(t, user.ID, shareSummary.Entries[0].Principal.ID)

	teamScoped, err := svc.ListVisible(context.Background(), ListConnectionsOptions{
		UserID:         user.ID,
		TeamID:         team.ID,
		IncludeTargets: false,
		IncludeGrants:  false,
	})
	require.NoError(t, err)
	require.Len(t, teamScoped.Connections, 1)
	require.Equal(t, "Team database", teamScoped.Connections[0].Name)

	personalScoped, err := svc.ListVisible(context.Background(), ListConnectionsOptions{
		UserID:         user.ID,
		TeamID:         "personal",
		IncludeTargets: false,
		IncludeGrants:  false,
	})
	require.NoError(t, err)
	require.Len(t, personalScoped.Connections, 2)
	namesPersonal := make([]string, 0, len(personalScoped.Connections))
	for _, conn := range personalScoped.Connections {
		namesPersonal = append(namesPersonal, conn.Name)
	}
	require.ElementsMatch(t, []string{"Primary SSH", "Kubernetes control plane"}, namesPersonal)
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

func TestConnectionServiceCreateWithTemplate(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	user := models.User{
		BaseModel: models.BaseModel{ID: "creator-1"},
		Username:  "creator",
		Email:     "creator@example.com",
		Password:  "secret",
	}
	require.NoError(t, db.Create(&user).Error)

	sections := []map[string]any{
		{
			"id":    "endpoint",
			"label": "Endpoint",
			"fields": []map[string]any{
				{
					"key":      "host",
					"label":    "Host",
					"type":     "string",
					"required": true,
					"binding": map[string]any{
						"target":   "target",
						"index":    0,
						"property": "host",
					},
				},
				{
					"key":     "port",
					"label":   "Port",
					"type":    "target_port",
					"default": 22,
					"binding": map[string]any{
						"target":   "target",
						"index":    0,
						"property": "port",
					},
				},
			},
		},
	}
	sectionsJSON, err := json.Marshal(sections)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.ConnectionTemplate{
		DriverID:    "ssh",
		Version:     "2025-01-15",
		DisplayName: "SSH Connection",
		Sections:    datatypes.JSON(sectionsJSON),
	}).Error)

	templateSvc, err := NewConnectionTemplateService(db, drivers.NewRegistry())
	require.NoError(t, err)

	svc, err := NewConnectionService(db, &mockPermissionChecker{
		grants: map[string]bool{
			"connection.create": true,
			"connection.view":   true,
		},
	}, WithConnectionTemplates(templateSvc))
	require.NoError(t, err)

	dto, err := svc.Create(context.Background(), user.ID, CreateConnectionInput{
		Name:        "Dynamic SSH",
		Description: "",
		ProtocolID:  "ssh",
		Fields: map[string]any{
			"host": "host.internal",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, dto)
	require.Equal(t, "Dynamic SSH", dto.Name)
	require.Equal(t, "ssh", dto.ProtocolID)
	require.Contains(t, dto.Metadata, "connection_template")

	var storedTargets []models.ConnectionTarget
	require.NoError(t, db.Where("connection_id = ?", dto.ID).Find(&storedTargets).Error)
	require.Len(t, storedTargets, 1)
	require.Equal(t, "host.internal", storedTargets[0].Host)
	require.Equal(t, 22, storedTargets[0].Port)
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

	personalCounts, err := svc.CountByFolder(context.Background(), ListConnectionsOptions{UserID: user.ID, TeamID: "personal"})
	require.NoError(t, err)
	require.Equal(t, int64(1), personalCounts["unassigned"])
	require.Equal(t, int64(2), personalCounts["folder-1"])
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

	personalCounts, err := svc.CountByProtocol(context.Background(), ListConnectionsOptions{
		UserID: user.ID,
		TeamID: "personal",
	})
	require.NoError(t, err)
	require.Zero(t, personalCounts["ssh"])
	require.Equal(t, int64(1), personalCounts["rdp"])
	require.Zero(t, personalCounts["postgres"])
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
