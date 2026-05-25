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
	mu     sync.RWMutex
	byName map[string]*entry
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]*entry)}
}

// Register validates a plugin's manifest + routes and adds it. It is safe for
// concurrent use and rejects duplicates.
func (r *Registry) Register(p Plugin) error {
	m := p.Manifest()
	routes := p.Routes()
	if err := Validate(m, routes); err != nil {
		return fmt.Errorf("plugin %q: %w", m.Name, err)
	}

	idx := make(map[string]Route, len(routes))
	for _, rt := range routes {
		idx[rt.ID] = rt
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[m.Name]; exists {
		return fmt.Errorf("plugin %q: %w", m.Name, ErrAlreadyExists)
	}
	r.byName[m.Name] = &entry{plugin: p, manifest: m, routes: idx}
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
			Description: e.manifest.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
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
