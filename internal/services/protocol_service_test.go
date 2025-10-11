package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/permissions"
)

type stubPermissionChecker struct {
	grants map[string]bool
	err    error
}

func (s *stubPermissionChecker) Check(ctx context.Context, userID, permissionID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.grants[permissionID], nil
}

func (s *stubPermissionChecker) CheckResource(ctx context.Context, userID, resourceType, resourceID, permissionID string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.grants[permissionID], nil
}

func TestProtocolServiceListAll(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureProtocolPermissionsRegistered(t)

	records := []models.ConnectionProtocol{
		marshalProtocolRecord(t, models.ConnectionProtocol{
			Name:          "SSH",
			ProtocolID:    "ssh",
			Module:        "ssh",
			Description:   "Secure Shell",
			Category:      "terminal",
			DefaultPort:   22,
			SortOrder:     1,
			DriverEnabled: true,
			ConfigEnabled: true,
		}, drivers.Capabilities{Terminal: true}),
		marshalProtocolRecord(t, models.ConnectionProtocol{
			Name:          "RDP",
			ProtocolID:    "rdp",
			Module:        "rdp",
			Description:   "Remote Desktop",
			Category:      "desktop",
			DefaultPort:   3389,
			SortOrder:     2,
			DriverEnabled: false,
			ConfigEnabled: true,
		}, drivers.Capabilities{Desktop: true}),
	}

	for _, record := range records {
		req := record
		require.NoError(t, db.Create(&req).Error)
	}

	svc, err := NewProtocolService(db, nil)
	require.NoError(t, err)

	infos, err := svc.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, infos, 2)
	require.Equal(t, "ssh", infos[0].ID)
	require.True(t, infos[0].Available)
	require.NotEmpty(t, infos[0].Permissions)
	require.False(t, infos[1].DriverEnabled)
}

func TestProtocolServiceListForUser(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureProtocolPermissionsRegistered(t)
	record := marshalProtocolRecord(t, models.ConnectionProtocol{
		Name:          "SSH",
		ProtocolID:    "ssh",
		Module:        "ssh",
		DriverEnabled: true,
		ConfigEnabled: true,
	}, drivers.Capabilities{Terminal: true})
	require.NoError(t, db.Create(&record).Error)

	checker := &stubPermissionChecker{grants: map[string]bool{
		"connection.view":      true,
		"protocol:ssh.connect": true,
	}}

	svc, err := NewProtocolService(db, checker)
	require.NoError(t, err)

	infos, err := svc.ListForUser(context.Background(), "user-123")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "ssh", infos[0].ID)
	require.NotEmpty(t, infos[0].Permissions)
}

func TestProtocolServiceListForUserPermissionDenied(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureProtocolPermissionsRegistered(t)
	record := marshalProtocolRecord(t, models.ConnectionProtocol{
		Name:          "SSH",
		ProtocolID:    "ssh",
		Module:        "ssh",
		DriverEnabled: true,
		ConfigEnabled: true,
	}, drivers.Capabilities{Terminal: true})
	require.NoError(t, db.Create(&record).Error)

	checker := &stubPermissionChecker{grants: map[string]bool{
		"connection.view": false,
	}}

	svc, _ := NewProtocolService(db, checker)
	infos, err := svc.ListForUser(context.Background(), "user-123")
	require.NoError(t, err)
	require.Empty(t, infos)
}

func TestProtocolServiceListForUserErrors(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureProtocolPermissionsRegistered(t)
	record := marshalProtocolRecord(t, models.ConnectionProtocol{
		Name:          "SSH",
		ProtocolID:    "ssh",
		Module:        "ssh",
		DriverEnabled: true,
		ConfigEnabled: true,
	}, drivers.Capabilities{Terminal: true})
	require.NoError(t, db.Create(&record).Error)

	svc, err := NewProtocolService(db, &stubPermissionChecker{err: errors.New("boom")})
	require.NoError(t, err)
	_, err = svc.ListForUser(context.Background(), "user-123")
	require.Error(t, err)
}

func TestProtocolServiceListPermissions(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())
	ensureProtocolPermissionsRegistered(t)
	record := marshalProtocolRecord(t, models.ConnectionProtocol{
		Name:          "SSH",
		ProtocolID:    "ssh",
		Module:        "ssh",
		DriverEnabled: true,
		ConfigEnabled: true,
	}, drivers.Capabilities{Terminal: true})
	require.NoError(t, db.Create(&record).Error)

	svc, err := NewProtocolService(db, nil)
	require.NoError(t, err)

	perms, err := svc.ListPermissions(context.Background(), "ssh")
	require.NoError(t, err)
	require.NotEmpty(t, perms)

	_, err = svc.ListPermissions(context.Background(), "unknown")
	require.Error(t, err)
}

func marshalProtocolRecord(t *testing.T, record models.ConnectionProtocol, caps drivers.Capabilities) models.ConnectionProtocol {
	t.Helper()

	features := deriveFeatures(caps)
	featuresJSON, err := json.Marshal(features)
	require.NoError(t, err)
	capsJSON, err := json.Marshal(caps)
	require.NoError(t, err)

	record.Features = datatypes.JSON(featuresJSON)
	record.Capabilities = datatypes.JSON(capsJSON)
	return record
}

func deriveFeatures(caps drivers.Capabilities) []string {
	features := make([]string, 0, 8)
	if caps.Terminal {
		features = append(features, "terminal")
	}
	if caps.Desktop {
		features = append(features, "desktop")
	}
	if caps.FileTransfer {
		features = append(features, "file_transfer")
	}
	if caps.Clipboard {
		features = append(features, "clipboard")
	}
	if caps.SessionRecording {
		features = append(features, "session_recording")
	}
	if caps.Metrics {
		features = append(features, "metrics")
	}
	if caps.Reconnect {
		features = append(features, "reconnect")
	}
	return features
}

func ensureProtocolPermissionsRegistered(t *testing.T) {
	t.Helper()

	if _, ok := permissions.Get("protocol:ssh.connect"); !ok {
		require.NoError(t, permissions.RegisterProtocolPermission("ssh", "connect", &permissions.Permission{
			DisplayName:  "SSH Connect",
			Description:  "Initiate SSH sessions",
			DefaultScope: "resource",
			DependsOn:    []string{"connection.launch"},
		}))
	}

	if _, ok := permissions.Get("protocol:ssh.port_forward"); !ok {
		require.NoError(t, permissions.RegisterProtocolPermission("ssh", "port_forward", &permissions.Permission{
			DisplayName:  "SSH Port Forward",
			Description:  "Forward ports through SSH tunnels",
			DefaultScope: "resource",
			DependsOn:    []string{"protocol:ssh.connect"},
		}))
	}
}
