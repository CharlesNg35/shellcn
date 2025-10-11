package drivers

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrNilDriver signals an attempt to register a nil driver instance.
	ErrNilDriver = errors.New("drivers: nil driver")
	// ErrEmptyDriverID indicates a driver descriptor with no identifier value.
	ErrEmptyDriverID = errors.New("drivers: descriptor id is required")
	// ErrDuplicateDriverID indicates a driver registration conflict.
	ErrDuplicateDriverID = errors.New("drivers: descriptor id already registered")
)

// Registry stores drivers keyed by descriptor ID with concurrency safety.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// NewRegistry constructs an empty driver registry instance.
func NewRegistry() *Registry {
	return &Registry{drivers: make(map[string]Driver)}
}

// Register adds a driver to the registry after descriptor validation.
func (r *Registry) Register(driver Driver) error {
	if driver == nil {
		return ErrNilDriver
	}

	desc := driver.Descriptor()
	id := strings.TrimSpace(desc.ID)
	if id == "" {
		return ErrEmptyDriverID
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.drivers[id]; exists {
		return ErrDuplicateDriverID
	}

	r.drivers[id] = driver
	return nil
}

// MustRegister wraps Register and panics on validation errors. Intended for init usage.
func (r *Registry) MustRegister(driver Driver) {
	if err := r.Register(driver); err != nil {
		panic(err)
	}
}

// Get returns the driver registered for id when present.
func (r *Registry) Get(id string) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	drv, ok := r.drivers[strings.TrimSpace(id)]
	return drv, ok
}

// MustGet fetches a driver or panics; helper for boot-time lookups.
func (r *Registry) MustGet(id string) Driver {
	drv, ok := r.Get(id)
	if !ok {
		panic("drivers: driver not registered: " + id)
	}
	return drv
}

// Describe returns a copy of known driver descriptors sorted by SortOrder then ID.
func (r *Registry) Describe(ctx context.Context) ([]Descriptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	descriptors := make([]Descriptor, 0, len(r.drivers))
	for _, driver := range r.drivers {
		descriptors = append(descriptors, driver.Descriptor())
	}

	sort.SliceStable(descriptors, func(i, j int) bool {
		if descriptors[i].SortOrder == descriptors[j].SortOrder {
			return descriptors[i].ID < descriptors[j].ID
		}
		return descriptors[i].SortOrder < descriptors[j].SortOrder
	})

	return descriptors, nil
}

// Capabilities returns the capability metadata for the specified driver.
func (r *Registry) Capabilities(ctx context.Context, id string) (Capabilities, error) {
	drv, ok := r.Get(id)
	if !ok {
		return Capabilities{}, errors.New("drivers: unknown driver " + id)
	}
	caps, err := drv.Capabilities(ctx)
	if err != nil {
		return Capabilities{}, err
	}
	if caps.Extras == nil {
		caps.Extras = map[string]bool{}
	}
	return caps, nil
}

// Reset clears the registry. Exported for testing only.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers = make(map[string]Driver)
}
