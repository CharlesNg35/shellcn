package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ErrProviderExists is returned when attempting to register a provider type more than once.
var ErrProviderExists = errors.New("provider registry: provider already registered")

// ProviderConfig bundles the persisted configuration for a provider required during instantiation.
type ProviderConfig struct {
	Type        string
	Name        string
	Description string
	Icon        string
	Enabled     bool
	// Raw contains the JSON configuration payload persisted for the provider.
	Raw json.RawMessage
	// Secrets contains decrypted secret material (client secrets, private keys, etc.).
	Secrets map[string]string
}

// Factory builds a concrete provider instance from configuration.
type Factory func(cfg ProviderConfig) (Provider, error)

// Descriptor describes a provider implementation the registry can expose.
type Descriptor struct {
	Metadata Metadata
	Factory  Factory
}

// Registry maintains a catalogue of known authentication provider implementations.
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
}

// NewRegistry constructs an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		descriptors: make(map[string]Descriptor),
	}
}

// Register adds a provider descriptor to the registry, enforcing uniqueness by provider type.
func (r *Registry) Register(desc Descriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := normaliseMetadata(desc.Metadata)
	if meta.Type == "" {
		return errors.New("provider registry: metadata type is required")
	}

	if _, exists := r.descriptors[meta.Type]; exists {
		return fmt.Errorf("%w: %s", ErrProviderExists, meta.Type)
	}

	r.descriptors[meta.Type] = Descriptor{
		Metadata: meta,
		Factory:  desc.Factory,
	}
	return nil
}

// Metadata returns all registered provider metadata ordered by their configured order and display name.
func (r *Registry) Metadata() []Metadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Metadata, 0, len(r.descriptors))
	for _, desc := range r.descriptors {
		items = append(items, desc.Metadata)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Order == items[j].Order {
			return items[i].DisplayName < items[j].DisplayName
		}
		return items[i].Order < items[j].Order
	})

	return items
}

// FactoryFor retrieves the factory function for the requested provider type, if registered.
func (r *Registry) FactoryFor(providerType string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	desc, ok := r.descriptors[strings.ToLower(providerType)]
	if !ok || desc.Factory == nil {
		return nil, false
	}
	return desc.Factory, true
}

func normaliseMetadata(meta Metadata) Metadata {
	meta.Type = strings.ToLower(strings.TrimSpace(meta.Type))
	meta.DisplayName = strings.TrimSpace(meta.DisplayName)
	meta.Description = strings.TrimSpace(meta.Description)
	meta.Icon = strings.TrimSpace(meta.Icon)
	meta.ButtonText = strings.TrimSpace(meta.ButtonText)
	if meta.Order == 0 {
		meta.Order = 100
	}
	return meta
}
