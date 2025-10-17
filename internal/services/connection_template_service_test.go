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

func TestConnectionTemplateServiceResolveFromDB(t *testing.T) {
	db := testutil.MustOpenTestDB(t, testutil.WithAutoMigrate())

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
			},
		},
	}
	sectionsJSON, err := json.Marshal(sections)
	require.NoError(t, err)
	metadataJSON, err := json.Marshal(map[string]any{"requires_identity": true})
	require.NoError(t, err)

	record := models.ConnectionTemplate{
		DriverID:    "ssh",
		Version:     "1.0.0",
		DisplayName: "SSH Connection",
		Sections:    datatypes.JSON(sectionsJSON),
		Metadata:    datatypes.JSON(metadataJSON),
	}
	require.NoError(t, db.Create(&record).Error)

	svc, err := NewConnectionTemplateService(db, drivers.NewRegistry())
	require.NoError(t, err)

	template, err := svc.Resolve(context.Background(), "ssh")
	require.NoError(t, err)
	require.NotNil(t, template)
	require.Equal(t, "1.0.0", template.Version)
	require.True(t, template.Metadata["requires_identity"].(bool))
}

func TestConnectionTemplateServiceMaterialise(t *testing.T) {
	svc := &ConnectionTemplateService{}

	template := &drivers.ConnectionTemplate{
		DriverID:    "ssh",
		Version:     "2025-01-15",
		DisplayName: "SSH",
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
					{
						Key:     "port",
						Label:   "Port",
						Type:    drivers.ConnectionFieldTypeTargetPort,
						Default: 22,
						Binding: &drivers.ConnectionBinding{
							Target:   drivers.BindingTargetConnectionTarget,
							Index:    0,
							Property: "port",
						},
					},
				},
			},
			{
				ID:    "session",
				Label: "Session",
				Fields: []drivers.ConnectionField{
					{
						Key:     "session_override_enabled",
						Label:   "Custom",
						Type:    drivers.ConnectionFieldTypeBoolean,
						Default: false,
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetMetadata,
							Path:   "session_override.enabled",
						},
					},
					{
						Key:          "concurrent_limit",
						Label:        "Concurrent",
						Type:         drivers.ConnectionFieldTypeNumber,
						Default:      0,
						Dependencies: []drivers.FieldDependency{{Field: "session_override_enabled", Equals: true}},
						Binding: &drivers.ConnectionBinding{
							Target: drivers.BindingTargetSettings,
							Path:   "concurrent_limit",
						},
					},
				},
			},
		},
		Metadata: map[string]any{"requires_identity": true},
	}

	fields := map[string]any{
		"host":                     "example.com",
		"session_override_enabled": true,
		"concurrent_limit":         3,
	}

	materialised, err := svc.Materialise(template, fields)
	require.NoError(t, err)
	require.NotNil(t, materialised)
	require.Equal(t, "2025-01-15", materialised.TemplateVersion)
	require.Equal(t, map[string]any{"concurrent_limit": 3}, materialised.Settings)
	require.Equal(t, true, materialised.Metadata["session_override"].(map[string]any)["enabled"])
	require.Len(t, materialised.Targets, 1)
	require.Equal(t, "example.com", materialised.Targets[0].Host)
	require.Equal(t, 22, materialised.Targets[0].Port)
}

func TestConnectionTemplateServiceMaterialiseMissingField(t *testing.T) {
	svc := &ConnectionTemplateService{}

	template := &drivers.ConnectionTemplate{
		DriverID: "ssh",
		Version:  "2025-01-15",
		Sections: []drivers.ConnectionSection{
			{
				ID: "endpoint",
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
	}

	_, err := svc.Materialise(template, map[string]any{})
	require.Error(t, err)
}
