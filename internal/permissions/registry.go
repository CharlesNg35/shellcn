package permissions

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Permission describes a permission definition registered by modules.
type Permission struct {
	ID          string
	Module      string
	DependsOn   []string
	Implies     []string
	Description string
}

type permissionRegistry struct {
	mu          sync.RWMutex
	permissions map[string]*Permission
}

var globalRegistry = &permissionRegistry{
	permissions: make(map[string]*Permission),
}

var (
	errNilPermission   = errors.New("permission: nil definition")
	errEmptyID         = errors.New("permission: id is required")
	errDuplicateID     = errors.New("permission: already registered")
	errSelfDependency  = errors.New("permission: cannot depend on itself")
	errSelfImplication = errors.New("permission: cannot imply itself")
)

// Register adds a permission definition to the global registry.
func Register(perm *Permission) error {
	if perm == nil {
		return errNilPermission
	}

	id := strings.TrimSpace(perm.ID)
	if id == "" {
		return errEmptyID
	}

	def := clonePermission(perm)
	def.ID = id
	def.Module = strings.TrimSpace(def.Module)

	depends, err := normaliseIDs(def.DependsOn, id, errSelfDependency)
	if err != nil {
		return err
	}
	implies, err := normaliseIDs(def.Implies, id, errSelfImplication)
	if err != nil {
		return err
	}
	def.DependsOn = depends
	def.Implies = implies

	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if _, exists := globalRegistry.permissions[id]; exists {
		return fmt.Errorf("%w: %s", errDuplicateID, id)
	}

	globalRegistry.permissions[id] = def
	return nil
}

// Get returns a copy of the permission definition when registered.
func Get(id string) (*Permission, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	perm, ok := globalRegistry.permissions[id]
	if !ok {
		return nil, false
	}
	return clonePermission(perm), true
}

// GetAll returns a copy of all registered permissions keyed by ID.
func GetAll() map[string]*Permission {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	out := make(map[string]*Permission, len(globalRegistry.permissions))
	for id, perm := range globalRegistry.permissions {
		out[id] = clonePermission(perm)
	}
	return out
}

// GetByModule gathers permissions registered under the specified module.
func GetByModule(module string) []*Permission {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	module = strings.TrimSpace(module)
	var perms []*Permission
	for _, perm := range globalRegistry.permissions {
		if perm.Module == module {
			perms = append(perms, clonePermission(perm))
		}
	}
	return perms
}

// ValidateDependencies ensures that all dependencies reference known permissions.
func ValidateDependencies() error {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	for _, perm := range globalRegistry.permissions {
		for _, dep := range perm.DependsOn {
			if _, ok := globalRegistry.permissions[dep]; !ok {
				return fmt.Errorf("permission: %s depends on unknown permission %s", perm.ID, dep)
			}
		}
	}
	return nil
}

func clonePermission(perm *Permission) *Permission {
	if perm == nil {
		return nil
	}

	cp := *perm
	if len(perm.DependsOn) > 0 {
		cp.DependsOn = append([]string(nil), perm.DependsOn...)
	}
	if len(perm.Implies) > 0 {
		cp.Implies = append([]string(nil), perm.Implies...)
	}
	return &cp
}

func normaliseIDs(values []string, self string, selfErr error) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(values))
	var result []string

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if value == self {
			return nil, selfErr
		}
		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result, nil
}

// reset clears registry entries. Intended for testing only.
func reset() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.permissions = make(map[string]*Permission)
}
