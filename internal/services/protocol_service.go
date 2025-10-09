package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/models"
)

// PermissionEvaluator matches permissions.Checker subset used by the protocol service.
type PermissionEvaluator interface {
	Check(ctx context.Context, userID, permissionID string) (bool, error)
}

// ProtocolInfo represents a protocol record returned to API consumers.
type ProtocolInfo struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Module        string               `json:"module"`
	Description   string               `json:"description"`
	Category      string               `json:"category"`
	Icon          string               `json:"icon"`
	DefaultPort   int                  `json:"default_port"`
	SortOrder     int                  `json:"sort_order"`
	Features      []string             `json:"features"`
	Capabilities  drivers.Capabilities `json:"capabilities"`
	DriverEnabled bool                 `json:"driver_enabled"`
	ConfigEnabled bool                 `json:"config_enabled"`
	Available     bool                 `json:"available"`
}

// ProtocolService exposes read operations for connection protocols.
type ProtocolService struct {
	db      *gorm.DB
	checker PermissionEvaluator
}

// NewProtocolService constructs a ProtocolService.
func NewProtocolService(db *gorm.DB, checker PermissionEvaluator) (*ProtocolService, error) {
	if db == nil {
		return nil, errors.New("protocol service: db is required")
	}
	return &ProtocolService{db: db, checker: checker}, nil
}

// ListAll returns all catalogued protocols without permission filtering.
func (s *ProtocolService) ListAll(ctx context.Context) ([]ProtocolInfo, error) {
	ctx = ensureContext(ctx)

	var rows []models.ConnectionProtocol
	if err := s.db.WithContext(ctx).
		Order("sort_order ASC, protocol_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("protocol service: list protocols: %w", err)
	}

	infos := make([]ProtocolInfo, 0, len(rows))
	for _, row := range rows {
		info, err := mapProtocolRow(row)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// ListForUser returns available protocols for the supplied user ID using permission checks.
func (s *ProtocolService) ListForUser(ctx context.Context, userID string) ([]ProtocolInfo, error) {
	ctx = ensureContext(ctx)
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("protocol service: user id is required")
	}

	protocols, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	if s.checker == nil {
		return filterAvailable(protocols), nil
	}

	viewAllowed, err := s.checker.Check(ctx, userID, "connection.view")
	if err != nil {
		return nil, fmt.Errorf("protocol service: permission check: %w", err)
	}
	if !viewAllowed {
		return []ProtocolInfo{}, nil
	}

	allowed := make([]ProtocolInfo, 0, len(protocols))
	for _, proto := range protocols {
		if !proto.Available {
			continue
		}
		permID := proto.ID + ".connect"
		ok, err := s.checker.Check(ctx, userID, permID)
		if err != nil {
			return nil, fmt.Errorf("protocol service: permission check %s: %w", permID, err)
		}
		if ok {
			allowed = append(allowed, proto)
		}
	}

	sort.SliceStable(allowed, func(i, j int) bool {
		if allowed[i].SortOrder == allowed[j].SortOrder {
			return allowed[i].ID < allowed[j].ID
		}
		return allowed[i].SortOrder < allowed[j].SortOrder
	})

	return allowed, nil
}

func filterAvailable(protocols []ProtocolInfo) []ProtocolInfo {
	filtered := make([]ProtocolInfo, 0, len(protocols))
	for _, proto := range protocols {
		if proto.Available {
			filtered = append(filtered, proto)
		}
	}
	return filtered
}

func mapProtocolRow(row models.ConnectionProtocol) (ProtocolInfo, error) {
	var features []string
	if strings.TrimSpace(row.Features) != "" {
		if err := json.Unmarshal([]byte(row.Features), &features); err != nil {
			return ProtocolInfo{}, fmt.Errorf("protocol service: decode features: %w", err)
		}
	}

	var caps drivers.Capabilities
	if strings.TrimSpace(row.Capabilities) != "" {
		if err := json.Unmarshal([]byte(row.Capabilities), &caps); err != nil {
			return ProtocolInfo{}, fmt.Errorf("protocol service: decode capabilities: %w", err)
		}
	}
	if caps.Extras == nil {
		caps.Extras = map[string]bool{}
	}

	available := row.DriverEnabled && row.ConfigEnabled

	return ProtocolInfo{
		ID:            row.ProtocolID,
		Name:          row.Name,
		Module:        row.Module,
		Description:   row.Description,
		Category:      row.Category,
		Icon:          row.Icon,
		DefaultPort:   row.DefaultPort,
		SortOrder:     row.SortOrder,
		Features:      features,
		Capabilities:  caps,
		DriverEnabled: row.DriverEnabled,
		ConfigEnabled: row.ConfigEnabled,
		Available:     available,
	}, nil
}
