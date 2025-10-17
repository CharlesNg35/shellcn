package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
)

// ProtocolCatalogService persists protocol metadata sourced from driver registries.
type ProtocolCatalogService struct {
	db *gorm.DB
}

// NewProtocolCatalogService constructs a ProtocolCatalogService instance.
func NewProtocolCatalogService(db *gorm.DB) (*ProtocolCatalogService, error) {
	if db == nil {
		return nil, errors.New("protocol catalog service: db is required")
	}
	return &ProtocolCatalogService{db: db}, nil
}

// Sync synchronises driver metadata into the connection_protocols table.
func (s *ProtocolCatalogService) Sync(ctx context.Context, driverReg *drivers.Registry, cfg *app.Config) error {
	if driverReg == nil {
		return errors.New("protocol catalog service: driver registry is required")
	}

	ctx = ensureContext(ctx)
	allDrivers := driverReg.All()
	tx := s.db.WithContext(ctx)

	for _, drv := range allDrivers {
		// Check driver health
		driverEnabled := true
		if reporter, ok := drv.(drivers.HealthReporter); ok {
			if err := reporter.HealthCheck(ctx); err != nil {
				driverEnabled = false
			}
		}

		// Get capabilities
		caps, err := drv.Capabilities(ctx)
		if err != nil {
			return fmt.Errorf("protocol catalog service: get capabilities for %s: %w", drv.ID(), err)
		}
		if caps.Extras == nil {
			caps.Extras = map[string]bool{}
		}

		// Check config
		configEnabled := protocolEnabled(cfg, drv.Module(), drv.ID())

		// Map capabilities to features
		features := mapCapabilitiesToFeatures(caps)
		featuresJSON, err := json.Marshal(features)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal features: %w", err)
		}
		capabilitiesJSON, err := json.Marshal(caps)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal capabilities: %w", err)
		}

		name := drv.Name()
		if strings.TrimSpace(name) == "" {
			name = strings.ToUpper(drv.ID())
		}

		description := drv.Description()
		defaultPort := drv.DefaultPort()

		record := models.ConnectionProtocol{
			Name:          name,
			ProtocolID:    drv.ID(),
			DriverID:      drv.ID(),
			Module:        drv.Module(),
			Icon:          drv.Icon(),
			Category:      drv.Category(),
			Description:   description,
			DefaultPort:   defaultPort,
			SortOrder:     drv.SortOrder(),
			Features:      datatypes.JSON(featuresJSON),
			Capabilities:  datatypes.JSON(capabilitiesJSON),
			DriverEnabled: driverEnabled,
			ConfigEnabled: configEnabled,
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "protocol_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "driver_id", "module", "icon", "category", "description", "default_port", "sort_order", "features", "capabilities", "driver_enabled", "config_enabled"}),
		}).Create(&record).Error; err != nil {
			return fmt.Errorf("protocol catalog service: upsert protocol %s: %w", drv.ID(), err)
		}

		if templater, ok := drv.(drivers.CredentialTemplater); ok {
			if err := s.persistCredentialTemplate(ctx, tx, drv.ID(), templater); err != nil {
				return err
			}
		}
		if templater, ok := drv.(drivers.ConnectionTemplater); ok {
			if err := s.persistConnectionTemplate(ctx, tx, drv.ID(), templater); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ProtocolCatalogService) persistCredentialTemplate(ctx context.Context, tx *gorm.DB, fallbackDriverID string, templater drivers.CredentialTemplater) error {
	template, err := templater.CredentialTemplate()
	if err != nil {
		return fmt.Errorf("protocol catalog service: credential template: %w", err)
	}
	if template == nil {
		return nil
	}

	driverID := strings.TrimSpace(template.DriverID)
	if driverID == "" {
		driverID = fallbackDriverID
	}
	version := strings.TrimSpace(template.Version)
	if version == "" {
		return fmt.Errorf("protocol catalog service: credential template for %s missing version", driverID)
	}
	if len(template.Fields) == 0 {
		return fmt.Errorf("protocol catalog service: credential template for %s has no fields", driverID)
	}
	if len(template.CompatibleProtocols) == 0 {
		return fmt.Errorf("protocol catalog service: credential template for %s has no compatible protocols", driverID)
	}

	fieldsJSON, err := json.Marshal(template.Fields)
	if err != nil {
		return fmt.Errorf("protocol catalog service: marshal credential fields for %s: %w", driverID, err)
	}
	compatJSON, err := json.Marshal(template.CompatibleProtocols)
	if err != nil {
		return fmt.Errorf("protocol catalog service: marshal credential protocols for %s: %w", driverID, err)
	}

	var metadataJSON datatypes.JSON
	if template.Metadata != nil {
		metadataBytes, err := json.Marshal(template.Metadata)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal credential metadata for %s: %w", driverID, err)
		}
		metadataJSON = metadataBytes
	}

	payload := map[string]any{
		"driver_id":            driverID,
		"version":              version,
		"display_name":         strings.TrimSpace(template.DisplayName),
		"description":          strings.TrimSpace(template.Description),
		"fields":               template.Fields,
		"compatible_protocols": template.CompatibleProtocols,
		"deprecated_after":     template.DeprecatedAfter,
		"metadata":             template.Metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("protocol catalog service: marshal credential template payload for %s: %w", driverID, err)
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])

	record := models.CredentialTemplate{
		DriverID:            driverID,
		Version:             version,
		DisplayName:         strings.TrimSpace(template.DisplayName),
		Description:         strings.TrimSpace(template.Description),
		Fields:              fieldsJSON,
		CompatibleProtocols: compatJSON,
		DeprecatedAfter:     template.DeprecatedAfter,
		Metadata:            metadataJSON,
		Hash:                hash,
	}

	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "driver_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "fields", "compatible_protocols", "deprecated_after", "metadata", "hash"}),
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("protocol catalog service: upsert credential template for %s: %w", driverID, err)
	}

	return nil
}

func (s *ProtocolCatalogService) persistConnectionTemplate(ctx context.Context, tx *gorm.DB, fallbackDriverID string, templater drivers.ConnectionTemplater) error {
	template, err := templater.ConnectionTemplate()
	if err != nil {
		return fmt.Errorf("protocol catalog service: connection template: %w", err)
	}
	if template == nil {
		return nil
	}

	driverID := strings.TrimSpace(template.DriverID)
	if driverID == "" {
		driverID = fallbackDriverID
	}
	version := strings.TrimSpace(template.Version)
	if version == "" {
		return fmt.Errorf("protocol catalog service: connection template for %s missing version", driverID)
	}
	if err := validateConnectionTemplate(template); err != nil {
		return fmt.Errorf("protocol catalog service: connection template for %s invalid: %w", driverID, err)
	}

	sectionsJSON, err := json.Marshal(template.Sections)
	if err != nil {
		return fmt.Errorf("protocol catalog service: marshal connection sections for %s: %w", driverID, err)
	}

	var metadataJSON datatypes.JSON
	if template.Metadata != nil {
		metadataBytes, err := json.Marshal(template.Metadata)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal connection metadata for %s: %w", driverID, err)
		}
		metadataJSON = metadataBytes
	}

	payload := map[string]any{
		"driver_id":    driverID,
		"version":      version,
		"display_name": strings.TrimSpace(template.DisplayName),
		"description":  strings.TrimSpace(template.Description),
		"sections":     template.Sections,
		"metadata":     template.Metadata,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("protocol catalog service: marshal connection template payload for %s: %w", driverID, err)
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])

	record := models.ConnectionTemplate{
		DriverID:    driverID,
		Version:     version,
		DisplayName: strings.TrimSpace(template.DisplayName),
		Description: strings.TrimSpace(template.Description),
		Sections:    sectionsJSON,
		Metadata:    metadataJSON,
		Hash:        hash,
	}

	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "driver_id"}, {Name: "version"}},
		DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "sections", "metadata", "hash"}),
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("protocol catalog service: upsert connection template for %s: %w", driverID, err)
	}

	return nil
}

func validateConnectionTemplate(template *drivers.ConnectionTemplate) error {
	if template == nil {
		return errors.New("template is nil")
	}
	if len(template.Sections) == 0 {
		return errors.New("no sections defined")
	}
	seenKeys := make(map[string]struct{})
	for _, section := range template.Sections {
		if len(section.Fields) == 0 {
			return fmt.Errorf("section %q has no fields", section.ID)
		}
		for _, field := range section.Fields {
			key := strings.TrimSpace(field.Key)
			if key == "" {
				return fmt.Errorf("section %q contains field with empty key", section.ID)
			}
			if _, exists := seenKeys[key]; exists {
				return fmt.Errorf("duplicate field key %q detected", key)
			}
			seenKeys[key] = struct{}{}
			if field.Binding != nil {
				binding := field.Binding
				switch binding.Target {
				case drivers.BindingTargetSettings, drivers.BindingTargetMetadata:
					if strings.TrimSpace(binding.Path) == "" {
						return fmt.Errorf("field %q missing binding path", key)
					}
				case drivers.BindingTargetConnectionTarget:
					if strings.TrimSpace(binding.Property) == "" {
						return fmt.Errorf("field %q missing binding property for target", key)
					}
					if binding.Index < 0 {
						return fmt.Errorf("field %q has invalid binding index", key)
					}
				default:
					if strings.TrimSpace(binding.Target) != "" {
						return fmt.Errorf("field %q has unsupported binding target %q", key, binding.Target)
					}
				}
			}
		}
	}
	return nil
}

func protocolEnabled(cfg *app.Config, module string, protocolID string) bool {
	if cfg == nil {
		return true
	}

	switch strings.TrimSpace(module) {
	case "ssh":
		return cfg.Protocols.SSH.Enabled
	case "telnet":
		return cfg.Protocols.Telnet.Enabled
	case "sftp":
		return cfg.Protocols.SFTP.Enabled
	case "rdp":
		return cfg.Protocols.RDP.Enabled
	case "vnc":
		return cfg.Protocols.VNC.Enabled
	case "docker":
		return cfg.Protocols.Docker.Enabled
	case "kubernetes":
		return cfg.Protocols.Kubernetes.Enabled
	case "database":
		if !cfg.Protocols.Database.Enabled {
			return false
		}
		switch protocolID {
		case "mysql":
			return cfg.Protocols.Database.MySQL
		case "postgres":
			return cfg.Protocols.Database.Postgres
		case "redis":
			return cfg.Protocols.Database.Redis
		case "mongodb":
			return cfg.Protocols.Database.MongoDB
		default:
			return true
		}
	case "proxmox":
		return cfg.Protocols.Proxmox.Enabled
	case "object_storage":
		return cfg.Protocols.ObjectStorage.Enabled
	default:
		return true
	}
}

func mapCapabilitiesToFeatures(caps drivers.Capabilities) []string {
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
	if len(caps.Extras) > 0 {
		keys := make([]string, 0, len(caps.Extras))
		for key, enabled := range caps.Extras {
			if enabled {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		features = append(features, keys...)
	}
	return features
}
