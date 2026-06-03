package service

import (
	"context"
	"fmt"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// ProtocolService manages the admin-configured availability of each protocol.
type ProtocolService struct {
	settings store.ProtocolSettingStore
}

func NewProtocolService(settings store.ProtocolSettingStore) *ProtocolService {
	return &ProtocolService{settings: settings}
}

// States returns stored availability keyed by protocol; absent ones default to enabled.
func (s *ProtocolService) States(ctx context.Context) (map[string]models.ProtocolAvailability, error) {
	rows, err := s.settings.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.ProtocolAvailability, len(rows))
	for _, r := range rows {
		out[r.Protocol] = r.Availability
	}
	return out, nil
}

// Set validates and persists a protocol's availability.
func (s *ProtocolService) Set(ctx context.Context, protocol string, a models.ProtocolAvailability) error {
	if protocol == "" {
		return fmt.Errorf("%w: protocol is required", plugin.ErrInvalidInput)
	}
	if !a.Valid() {
		return fmt.Errorf("%w: unknown availability %q", plugin.ErrInvalidInput, a)
	}
	return s.settings.Set(ctx, &models.ProtocolSetting{Protocol: protocol, Availability: a})
}

// Allowed reports whether the protocol is usable by a user with the given role.
func (s *ProtocolService) Allowed(ctx context.Context, protocol string, isAdmin bool) (bool, error) {
	states, err := s.States(ctx)
	if err != nil {
		return false, err
	}
	return states[protocol].Allows(isAdmin), nil
}
