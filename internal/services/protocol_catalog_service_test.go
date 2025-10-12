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
)

type mockDriver struct {
	drivers.BaseDriver
	caps        drivers.Capabilities
	healthError error
}

func newMockDriver(desc drivers.Descriptor, caps drivers.Capabilities, healthErr error) *mockDriver {
	return &mockDriver{
		BaseDriver:  drivers.NewBaseDriver(desc),
		caps:        caps,
		healthError: healthErr,
	}
}

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
	driverReg.MustRegister(newMockDriver(
		drivers.Descriptor{ID: "ssh", Module: "ssh", Title: "Secure Shell", SortOrder: 1},
		drivers.Capabilities{Terminal: true, FileTransfer: true},
		nil,
	))
	driverReg.MustRegister(newMockDriver(
		drivers.Descriptor{ID: "rdp", Module: "rdp", Title: "Remote Desktop", SortOrder: 2},
		drivers.Capabilities{Desktop: true},
		errors.New("rdp driver offline"),
	))

	cfg := &app.Config{}
	cfg.Protocols.SSH.Enabled = true
	cfg.Protocols.RDP.Enabled = false

	svc, err := NewProtocolCatalogService(db)
	require.NoError(t, err)
	require.NoError(t, svc.Sync(context.Background(), driverReg, cfg))

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
	require.NoError(t, json.Unmarshal(sshRecord.Features, &sshFeatures))
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
