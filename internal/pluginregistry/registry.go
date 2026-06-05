// Package pluginregistry owns the gateway runtime registry for built-in and
// external plugin instances.
package pluginregistry

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type entry struct {
	plugin   plugin.Plugin
	manifest plugin.Manifest
	routes   map[string]plugin.Route
}

type Registry struct {
	mu              sync.RWMutex
	byName          map[string]*entry
	credentialKinds *plugin.CredentialKindSet
}

func New() *Registry {
	return &Registry{
		byName:          make(map[string]*entry),
		credentialKinds: plugin.MustCredentialKindSet(plugin.BuiltInCredentialKinds()),
	}
}

func (r *Registry) Register(p plugin.Plugin) error {
	m := p.Manifest()
	routes := p.Routes()

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[m.Name]; exists {
		return fmt.Errorf("plugin %q: %w", m.Name, plugin.ErrAlreadyExists)
	}
	catalog := r.credentialKinds.Clone()
	if err := plugin.ValidateWithCredentialKinds(m, routes, catalog); err != nil {
		return fmt.Errorf("plugin %q: %w", m.Name, err)
	}
	for _, info := range m.CredentialKinds {
		if err := r.credentialKinds.Add(info); err != nil {
			return fmt.Errorf("plugin %q: %w", m.Name, err)
		}
	}
	plugin.AddCredentialKindSupports(r.credentialKinds, m)

	r.byName[m.Name] = &entry{plugin: p, manifest: m, routes: routeMap(routes)}
	return nil
}

func (r *Registry) Replace(p plugin.Plugin) error {
	m := p.Manifest()
	routes := p.Routes()

	r.mu.Lock()
	defer r.mu.Unlock()
	old, exists := r.byName[m.Name]
	if !exists {
		return fmt.Errorf("plugin %q: %w", m.Name, plugin.ErrNotFound)
	}

	ownKinds := map[plugin.CredentialKind]bool{}
	for _, info := range old.manifest.CredentialKinds {
		ownKinds[normalizeKind(info.Kind)] = true
	}
	if err := plugin.ValidateWithCredentialKinds(m, routes, r.credentialKinds.CloneWithout(ownKinds)); err != nil {
		return fmt.Errorf("plugin %q: %w", m.Name, err)
	}
	for _, info := range m.CredentialKinds {
		if _, known := r.credentialKinds.CredentialKindLookup(normalizeKind(info.Kind)); known {
			continue
		}
		if err := r.credentialKinds.Add(info); err != nil {
			return fmt.Errorf("plugin %q: %w", m.Name, err)
		}
	}
	plugin.AddCredentialKindSupports(r.credentialKinds, m)

	r.byName[m.Name] = &entry{plugin: p, manifest: m, routes: routeMap(routes)}
	return nil
}

func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[name]; !exists {
		return fmt.Errorf("plugin %q: %w", name, plugin.ErrNotFound)
	}
	delete(r.byName, name)
	return nil
}

func (r *Registry) MustRegister(p plugin.Plugin) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

func (r *Registry) Get(name string) (plugin.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return nil, false
	}
	return e.plugin, true
}

func (r *Registry) Manifest(name string) (plugin.Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return plugin.Manifest{}, false
	}
	return e.manifest, true
}

func (r *Registry) Route(pluginName, routeID string) (plugin.Route, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[pluginName]
	if !ok {
		return plugin.Route{}, false
	}
	rt, ok := e.routes[routeID]
	return rt, ok
}

func (r *Registry) All() []plugin.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]plugin.Plugin, 0, len(r.byName))
	for _, e := range r.byName {
		out = append(out, e.plugin)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Manifest().Name < out[j].Manifest().Name
	})
	return out
}

func (r *Registry) Summaries() []plugin.Summary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]plugin.Summary, 0, len(r.byName))
	for _, e := range r.byName {
		category, _ := plugin.CategoryLookup(e.manifest.Category)
		out = append(out, plugin.Summary{
			Name:        e.manifest.Name,
			Title:       e.manifest.Title,
			Icon:        e.manifest.Icon,
			Category:    category,
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

func (r *Registry) Projection(name string) (plugin.Projection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	if !ok {
		return plugin.Projection{}, false
	}
	return plugin.BuildProjection(e.manifest, e.routes), true
}

func (r *Registry) CredentialKinds() []plugin.CredentialKindInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKinds()
}

func (r *Registry) CredentialKindLookup(kind plugin.CredentialKind) (plugin.CredentialKindInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKindLookup(kind)
}

func (r *Registry) CredentialKindSupportsProtocol(kind plugin.CredentialKind, protocol string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.credentialKinds.CredentialKindSupportsProtocol(kind, protocol)
}

func routeMap(routes []plugin.Route) map[string]plugin.Route {
	out := make(map[string]plugin.Route, len(routes))
	for _, rt := range routes {
		out[rt.ID] = rt
	}
	return out
}

func normalizeKind(kind plugin.CredentialKind) plugin.CredentialKind {
	return plugin.CredentialKind(strings.TrimSpace(string(kind)))
}
