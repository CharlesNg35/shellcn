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
	"github.com/charlesng35/shellcn/internal/permissions"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
)

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
	Permissions   []ProtocolPermission `json:"permissions"`
}

// ProtocolPermission describes permission metadata associated with a protocol/driver.
type ProtocolPermission struct {
	ID           string         `json:"id"`
	DisplayName  string         `json:"display_name"`
	Description  string         `json:"description"`
	Category     string         `json:"category"`
	DefaultScope string         `json:"default_scope"`
	Module       string         `json:"module"`
	DependsOn    []string       `json:"depends_on,omitempty"`
	Implies      []string       `json:"implies,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ProtocolService exposes read operations for connection protocols.
type ProtocolService struct {
	db      *gorm.DB
	checker PermissionChecker
}

// NewProtocolService constructs a ProtocolService.
func NewProtocolService(db *gorm.DB, checker PermissionChecker) (*ProtocolService, error) {
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
		permID := permissionIDForProtocol(proto.ID, "connect")
		if permID == "" {
			continue
		}
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
	if len(row.Features) > 0 {
		if err := json.Unmarshal(row.Features, &features); err != nil {
			return ProtocolInfo{}, fmt.Errorf("protocol service: decode features: %w", err)
		}
	}

	var caps drivers.Capabilities
	if len(row.Capabilities) > 0 {
		if err := json.Unmarshal(row.Capabilities, &caps); err != nil {
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
		Permissions:   mapProtocolPermissions(row.ProtocolID),
	}, nil
}

func mapProtocolPermissions(protocolID string) []ProtocolPermission {
	if strings.TrimSpace(protocolID) == "" {
		return nil
	}

	registry := permissions.GetAll()
	prefix := "protocol:" + protocolID + "."

	perms := make([]ProtocolPermission, 0, len(registry))
	for _, def := range registry {
		if def == nil {
			continue
		}

		id := strings.TrimSpace(def.ID)
		if !strings.HasPrefix(id, prefix) {
			if driver, ok := def.Metadata["driver"].(string); ok && driver == protocolID {
				// allow metadata override in case ID naming diverges
			} else {
				continue
			}
		}

		entry := ProtocolPermission{
			ID:           id,
			DisplayName:  def.DisplayName,
			Description:  def.Description,
			Category:     def.Category,
			DefaultScope: def.DefaultScope,
			Module:       def.Module,
			DependsOn:    append([]string(nil), def.DependsOn...),
			Implies:      append([]string(nil), def.Implies...),
		}
		if len(def.Metadata) > 0 {
			metaCopy := make(map[string]any, len(def.Metadata))
			for k, v := range def.Metadata {
				metaCopy[k] = v
			}
			entry.Metadata = metaCopy
		}
		perms = append(perms, entry)
	}

	sort.Slice(perms, func(i, j int) bool {
		return perms[i].ID < perms[j].ID
	})

	return perms
}

func permissionIDForProtocol(protocolID, action string) string {
	protocolID = strings.TrimSpace(protocolID)
	action = strings.TrimSpace(action)
	if protocolID == "" || action == "" {
		return ""
	}
	return "protocol:" + protocolID + "." + action
}

// ListPermissions returns registered permission metadata for a protocol.
func (s *ProtocolService) ListPermissions(ctx context.Context, protocolID string) ([]ProtocolPermission, error) {
	ctx = ensureContext(ctx)
	protocolID = strings.TrimSpace(protocolID)
	if protocolID == "" {
		return nil, apperrors.NewBadRequest("protocol id is required")
	}

	var exists models.ConnectionProtocol
	if err := s.db.WithContext(ctx).
		Select("protocol_id").
		First(&exists, "protocol_id = ?", protocolID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("protocol service: load protocol: %w", err)
	}

	perms := mapProtocolPermissions(protocolID)
	if perms == nil {
		return []ProtocolPermission{}, nil
	}
	return perms, nil
}
