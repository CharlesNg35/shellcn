package protocols

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charlesng35/shellcn/internal/drivers"
)

var (
	errNilProtocol    = errors.New("protocols: nil definition")
	errEmptyProtocol  = errors.New("protocols: id is required")
	errEmptyDriver    = errors.New("protocols: driver id is required")
	errDuplicateProto = errors.New("protocols: already registered")
)

// Registry manages protocol definitions derived from driver descriptors.
type Registry struct {
	mu        sync.RWMutex
	protocols map[string]*Protocol
}

// NewRegistry constructs an empty protocol registry.
func NewRegistry() *Registry {
	return &Registry{protocols: make(map[string]*Protocol)}
}

// Register adds a protocol definition into the registry.
func (r *Registry) Register(proto *Protocol) error {
	if proto == nil {
		return errNilProtocol
	}
	id := strings.TrimSpace(proto.ID)
	if id == "" {
		return errEmptyProtocol
	}
	driverID := strings.TrimSpace(proto.DriverID)
	if driverID == "" {
		return errEmptyDriver
	}

	clone := cloneProtocol(proto)
	clone.ID = id
	clone.DriverID = driverID
	clone.Module = strings.TrimSpace(clone.Module)
	clone.Title = strings.TrimSpace(clone.Title)
	clone.Category = strings.TrimSpace(clone.Category)
	clone.Icon = strings.TrimSpace(clone.Icon)

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.protocols[id]; exists {
		return fmt.Errorf("%w: %s", errDuplicateProto, id)
	}
	r.protocols[id] = clone
	return nil
}

// MustRegister wraps Register and panics on error for init-time declarations.
func (r *Registry) MustRegister(proto *Protocol) {
	if err := r.Register(proto); err != nil {
		panic(err)
	}
}

// RegisterFromDriver builds a protocol definition from a driver descriptor/capabilities pair.
func (r *Registry) RegisterFromDriver(desc drivers.Descriptor, caps drivers.Capabilities) error {
	proto := &Protocol{
		ID:           strings.TrimSpace(desc.ID),
		DriverID:     strings.TrimSpace(desc.ID),
		Module:       strings.TrimSpace(desc.Module),
		Title:        strings.TrimSpace(desc.Title),
		Category:     strings.TrimSpace(desc.Category),
		Icon:         strings.TrimSpace(desc.Icon),
		SortOrder:    desc.SortOrder,
		Features:     mapCapabilitiesToFeatures(caps),
		Capabilities: caps,
	}
	return r.Register(proto)
}

// Get fetches a protocol definition copy.
func (r *Registry) Get(id string) (*Protocol, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	proto, ok := r.protocols[strings.TrimSpace(id)]
	if !ok {
		return nil, false
	}
	return cloneProtocol(proto), true
}

// GetAll returns protocol definitions sorted by SortOrder then ID.
func (r *Registry) GetAll() []*Protocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*Protocol, 0, len(r.protocols))
	for _, proto := range r.protocols {
		list = append(list, cloneProtocol(proto))
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].SortOrder == list[j].SortOrder {
			return list[i].ID < list[j].ID
		}
		return list[i].SortOrder < list[j].SortOrder
	})
	return list
}

// Reset clears the registry. Intended for test use only.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.protocols = make(map[string]*Protocol)
}

// SyncFromDrivers populates the registry from the provided driver registry.
func (r *Registry) SyncFromDrivers(ctx context.Context, driverRegistry *drivers.Registry) error {
	if driverRegistry == nil {
		return errors.New("protocols: driver registry is required")
	}
	descriptors, err := driverRegistry.Describe(ctx)
	if err != nil {
		return err
	}
	for _, desc := range descriptors {
		caps, err := driverRegistry.Capabilities(ctx, desc.ID)
		if err != nil {
			return err
		}
		if err := r.RegisterFromDriver(desc, caps); err != nil {
			return err
		}
	}
	return nil
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
