package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// entry is a validated, indexed plugin.
type entry struct {
	plugin   Plugin
	manifest Manifest
	routes   map[string]Route // keyed by Route.ID
}

// Registry holds the compiled-in plugins. It validates each manifest on
// registration and indexes routes by id for fast resolution.
type Registry struct {
	mu              sync.RWMutex
	byName          map[string]*entry
	credentialKinds *credentialKindSet
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byName:          make(map[string]*entry),
		credentialKinds: mustCredentialKindSet(builtInCredentialKindCatalog),
	}
}

// Register validates a plugin's manifest + routes and adds it. It is safe for
// concurrent use and rejects duplicates.
func (r *Registry) Register(p Plugin) error {
	m := p.Manifest()
	routes := p.Routes()

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[m.Name]; exists {
		return fmt.Errorf("plugin %q: %w", m.Name, ErrAlreadyExists)
	}
	catalog := r.credentialKinds.clone()
	if err := ValidateWithCredentialKinds(m, routes, catalog); err != nil {
		return fmt.Errorf("plugin %q: %w", m.Name, err)
	}
	for _, info := range m.CredentialKinds {
		if err := r.credentialKinds.add(info); err != nil {
			return fmt.Errorf("plugin %q: %w", m.Name, err)
		}
	}
	addCredentialKindSupports(r.credentialKinds, m)

	idx := make(map[string]Route, len(routes))
	for _, rt := range routes {
		idx[rt.ID] = rt
	}

	r.byName[m.Name] = &entry{plugin: p, manifest: m, routes: idx}
	return nil
}

// Replace swaps an already-registered plugin for a new instance with the same
// name (an external plugin update). Validation runs against the catalog minus
// the old version's own credential kinds so a plugin may keep re-declaring
// them; kinds are never removed (existing connections may reference them).
func (r *Registry) Replace(p Plugin) error {
	m := p.Manifest()
	routes := p.Routes()

	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.byName[m.Name]
	if !exists {
		return fmt.Errorf("plugin %q: %w", m.Name, ErrNotFound)
	}

	ownKinds := map[CredentialKind]bool{}
	for _, info := range old.manifest.CredentialKinds {
		ownKinds[normalizeCredentialKindInfo(info).Kind] = true
	}
	if err := ValidateWithCredentialKinds(m, routes, r.credentialKinds.cloneWithout(ownKinds)); err != nil {
		return fmt.Errorf("plugin %q: %w", m.Name, err)
	}
	for _, info := range m.CredentialKinds {
		if _, known := r.credentialKinds.byID[normalizeCredentialKindInfo(info).Kind]; known {
			continue
		}
		if err := r.credentialKinds.add(info); err != nil {
			return fmt.Errorf("plugin %q: %w", m.Name, err)
		}
	}
	addCredentialKindSupports(r.credentialKinds, m)

	idx := make(map[string]Route, len(routes))
	for _, rt := range routes {
		idx[rt.ID] = rt
	}
	r.byName[m.Name] = &entry{plugin: p, manifest: m, routes: idx}
	return nil
}

// Unregister removes one plugin entry. Credential kind metadata is intentionally
// kept because existing saved connections may still reference those kinds.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[name]; !exists {
		return fmt.Errorf("plugin %q: %w", name, ErrNotFound)
	}
	delete(r.byName, name)
	return nil
}

// MustRegister panics on registration failure — for wiring at startup.
func (r *Registry) MustRegister(p Plugin) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// Get returns the plugin singleton by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return nil, false
	}
	return e.plugin, true
}

// Manifest returns the (already validated) manifest by name.
func (r *Registry) Manifest(name string) (Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return Manifest{}, false
	}
	return e.manifest, true
}

// Route resolves a plugin's route by id.
func (r *Registry) Route(plugin, routeID string) (Route, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[plugin]
	if !ok {
		return Route{}, false
	}
	rt, ok := e.routes[routeID]
	return rt, ok
}

// All returns every registered plugin, ordered by name.
func (r *Registry) All() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.byName))
	for _, e := range r.byName {
		out = append(out, e.plugin)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest().Name < out[j].Manifest().Name
	})
	return out
}

// Summaries returns the lightweight catalog the connection list needs.
func (r *Registry) Summaries() []Summary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Summary, 0, len(r.byName))
	for _, e := range r.byName {
		out = append(out, Summary{
			Name:        e.manifest.Name,
			Title:       e.manifest.Title,
			Icon:        e.manifest.Icon,
			Category:    pluginCategoryInfo(e.manifest.Category),
			Description: e.manifest.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category.Order != out[j].Category.Order {
			return out[i].Category.Order < out[j].Category.Order
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Projection returns the render-only projection for a plugin by name.
func (r *Registry) Projection(name string) (Projection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return Projection{}, false
	}
	return BuildProjection(e.manifest, e.routes), true
}

// CredentialKinds returns every registered credential kind: core shared kinds
// followed by plugin-declared kinds in plugin registration order.
func (r *Registry) CredentialKinds() []CredentialKindInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKinds()
}

// CredentialKindLookup returns one credential kind's metadata.
func (r *Registry) CredentialKindLookup(kind CredentialKind) (CredentialKindInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKindLookup(kind)
}

// CredentialKindSupportsProtocol reports whether a credential kind may be
// explicitly scoped to protocol.
func (r *Registry) CredentialKindSupportsProtocol(kind CredentialKind, protocol string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKindSupportsProtocol(kind, protocol)
}

func addCredentialKindSupports(catalog *credentialKindSet, m Manifest) {
	for _, group := range m.Config.Groups {
		for _, field := range group.Fields {
			if field.Type != FieldCredentialRef || field.Credential == nil {
				continue
			}
			protocols := field.Credential.Protocols
			if len(protocols) == 0 && m.Name != "" {
				protocols = []string{m.Name}
			}
			for _, kind := range field.Credential.Kinds {
				for _, protocol := range protocols {
					catalog.addSupport(kind, protocol)
				}
			}
		}
	}
}
