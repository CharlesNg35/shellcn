package services

import (
	"context"
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
