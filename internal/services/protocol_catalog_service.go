package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/protocols"
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

// Sync synchronises the protocol registry into the connection_protocols table.
func (s *ProtocolCatalogService) Sync(ctx context.Context, protoReg *protocols.Registry, driverReg *drivers.Registry, cfg *app.Config) error {
	if protoReg == nil {
		return errors.New("protocol catalog service: protocol registry is required")
	}

	ctx = ensureContext(ctx)
	records := protoReg.GetAll()
	tx := s.db.WithContext(ctx)

	for _, proto := range records {
		driverEnabled := true
		if driverReg != nil {
			if drv, ok := driverReg.Get(proto.DriverID); ok {
				if reporter, ok := drv.(drivers.HealthReporter); ok {
					if err := reporter.HealthCheck(ctx); err != nil {
						driverEnabled = false
					}
				}
			}
		}

		configEnabled := protocolEnabled(cfg, proto.Module, proto.ID)

		featuresJSON, err := json.Marshal(proto.Features)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal features: %w", err)
		}
		capabilitiesJSON, err := json.Marshal(proto.Capabilities)
		if err != nil {
			return fmt.Errorf("protocol catalog service: marshal capabilities: %w", err)
		}

		name := proto.Title
		if strings.TrimSpace(name) == "" {
			name = strings.ToUpper(proto.ID)
		}

		record := models.ConnectionProtocol{
			Name:          name,
			ProtocolID:    proto.ID,
			DriverID:      proto.DriverID,
			Module:        proto.Module,
			Icon:          proto.Icon,
			Category:      proto.Category,
			Description:   proto.Description,
			DefaultPort:   proto.DefaultPort,
			SortOrder:     proto.SortOrder,
			Features:      datatypes.JSON(featuresJSON),
			Capabilities:  datatypes.JSON(capabilitiesJSON),
			DriverEnabled: driverEnabled,
			ConfigEnabled: configEnabled,
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "protocol_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "driver_id", "module", "icon", "category", "description", "default_port", "sort_order", "features", "capabilities", "driver_enabled", "config_enabled"}),
		}).Create(&record).Error; err != nil {
			return fmt.Errorf("protocol catalog service: upsert protocol %s: %w", proto.ID, err)
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
