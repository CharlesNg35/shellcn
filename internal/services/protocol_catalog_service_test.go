package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/protocols"
)

type mockDriver struct {
	desc        drivers.Descriptor
	caps        drivers.Capabilities
	healthError error
}

func (m *mockDriver) Descriptor() drivers.Descriptor { return m.desc }

func (m *mockDriver) Capabilities(ctx context.Context) (drivers.Capabilities, error) {
	if m.caps.Extras == nil {
		m.caps.Extras = map[string]bool{}
	}
	return m.caps, nil
}

func (m *mockDriver) HealthCheck(ctx context.Context) error { return m.healthError }

func TestProtocolCatalogSyncPersistsRecords(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	driverReg := drivers.NewRegistry()
	driverReg.MustRegister(&mockDriver{
		desc: drivers.Descriptor{ID: "ssh", Module: "ssh", Title: "Secure Shell", SortOrder: 1},
		caps: drivers.Capabilities{Terminal: true, FileTransfer: true},
	})
	driverReg.MustRegister(&mockDriver{
		desc:        drivers.Descriptor{ID: "rdp", Module: "rdp", Title: "Remote Desktop", SortOrder: 2},
		caps:        drivers.Capabilities{Desktop: true},
		healthError: errors.New("rdp driver offline"),
	})

	protoReg := protocols.NewRegistry()
	require.NoError(t, protoReg.SyncFromDrivers(context.Background(), driverReg))

	cfg := &app.Config{}
	cfg.Modules.SSH.Enabled = true
	cfg.Modules.RDP.Enabled = false

	svc, err := NewProtocolCatalogService(db)
	require.NoError(t, err)
	require.NoError(t, svc.Sync(context.Background(), protoReg, driverReg, cfg))

	var records []models.ConnectionProtocol
	require.NoError(t, db.Find(&records).Error)
	require.Len(t, records, 2)

	results := map[string]models.ConnectionProtocol{}
	for _, record := range records {
		results[record.ProtocolID] = record
	}

	sshRecord := results["ssh"]
	require.True(t, sshRecord.DriverEnabled)
	require.True(t, sshRecord.ConfigEnabled)

	rdpRecord := results["rdp"]
	require.False(t, rdpRecord.DriverEnabled)
	require.False(t, rdpRecord.ConfigEnabled)

	var sshFeatures []string
	require.NoError(t, json.Unmarshal([]byte(sshRecord.Features), &sshFeatures))
	require.Contains(t, sshFeatures, "terminal")
	require.Contains(t, sshFeatures, "file_transfer")
}

type fkRow struct {
	ID       int
	Seq      int
	Table    string
	From     string
	To       string
	OnUpdate string `gorm:"column:on_update"`
	OnDelete string `gorm:"column:on_delete"`
	Match    string
}

func TestConnectionProtocolManualInsert(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	var fkRows []fkRow
	require.NoError(t, db.Raw("PRAGMA foreign_key_list(connection_protocols)").Scan(&fkRows).Error)
	t.Logf("connection_protocols foreign keys: %#v", fkRows)

	record := models.ConnectionProtocol{
		Name:        "SSH",
		ProtocolID:  "ssh",
		DriverID:    "ssh",
		Module:      "ssh",
		Description: "Secure Shell",
	}

	err := db.Create(&record).Error
	require.NoError(t, err)
}
