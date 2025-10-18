package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/database/testutil"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
)

type mockDriver struct {
	drivers.BaseDriver
	caps               drivers.Capabilities
	healthError        error
	credentialTemplate *drivers.CredentialTemplate
	connectionTemplate *drivers.ConnectionTemplate
}

func newMockDriver(desc drivers.Descriptor, caps drivers.Capabilities, healthErr error, credTemplate *drivers.CredentialTemplate, connTemplate *drivers.ConnectionTemplate) *mockDriver {
	return &mockDriver{
		BaseDriver:         drivers.NewBaseDriver(desc),
		caps:               caps,
		healthError:        healthErr,
		credentialTemplate: credTemplate,
		connectionTemplate: connTemplate,
	}
}

func (m *mockDriver) Capabilities(ctx context.Context) (drivers.Capabilities, error) {
	if m.caps.Extras == nil {
		m.caps.Extras = map[string]bool{}
	}
	return m.caps, nil
}

func (m *mockDriver) HealthCheck(ctx context.Context) error { return m.healthError }

func (m *mockDriver) CredentialTemplate() (*drivers.CredentialTemplate, error) {
	return m.credentialTemplate, nil
}

func (m *mockDriver) ConnectionTemplate() (*drivers.ConnectionTemplate, error) {
	return m.connectionTemplate, nil
}

func TestProtocolCatalogSyncPersistsRecords(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

	expiry := time.Now().Add(30 * 24 * time.Hour).UTC()

	driverReg := drivers.NewRegistry()
	driverReg.MustRegister(newMockDriver(
		drivers.Descriptor{ID: "ssh", Module: "ssh", Title: "Secure Shell", SortOrder: 1},
		drivers.Capabilities{Terminal: true, FileTransfer: true},
		nil,
		&drivers.CredentialTemplate{
			DriverID:    "ssh",
			Version:     "1.0.0",
			DisplayName: "SSH Credentials",
			Description: "Credentials for SSH connections",
			Fields: []drivers.CredentialField{
				{
					Key:         "username",
					Label:       "Username",
					Type:        "string",
					Required:    true,
					Description: "Login username",
					InputModes:  []string{"text"},
				},
				{
					Key:         "private_key",
					Label:       "Private Key",
					Type:        "secret",
					Required:    true,
					Description: "PEM encoded private key",
					InputModes:  []string{"text", "file"},
				},
			},
			CompatibleProtocols: []string{"ssh"},
			DeprecatedAfter:     &expiry,
			Metadata: map[string]any{
				"supports_otp": false,
			},
		},
		&drivers.ConnectionTemplate{
			DriverID:    "ssh",
			Version:     "2025-01-15",
			DisplayName: "SSH Connection",
			Description: "Connection schema for SSH",
			Sections: []drivers.ConnectionSection{
				{
					ID:    "endpoint",
					Label: "Endpoint",
					Fields: []drivers.ConnectionField{
						{
							Key:      "host",
							Label:    "Host",
							Type:     drivers.ConnectionFieldTypeString,
							Required: true,
							Binding: &drivers.ConnectionBinding{
								Target:   drivers.BindingTargetConnectionTarget,
								Index:    0,
								Property: "host",
							},
						},
					},
				},
			},
			Metadata: map[string]any{
				"requires_identity": true,
			},
		},
	))
	driverReg.MustRegister(newMockDriver(
		drivers.Descriptor{ID: "rdp", Module: "rdp", Title: "Remote Desktop", SortOrder: 2},
		drivers.Capabilities{Desktop: true},
		errors.New("rdp driver offline"),
		nil,
		nil,
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

	var templates []models.CredentialTemplate
	require.NoError(t, db.Find(&templates).Error)
	require.Len(t, templates, 1)

	template := templates[0]
	require.Equal(t, "ssh", template.DriverID)
	require.Equal(t, "1.0.0", template.Version)
	require.NotEmpty(t, template.Hash)
	require.NotNil(t, template.DeprecatedAfter)
	require.True(t, template.DeprecatedAfter.After(time.Now().Add(24*time.Hour)))

	var connTemplates []models.ConnectionTemplate
	require.NoError(t, db.Find(&connTemplates).Error)
	require.Len(t, connTemplates, 1)
	require.Equal(t, "ssh", connTemplates[0].DriverID)
	require.Equal(t, "2025-01-15", connTemplates[0].Version)
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
