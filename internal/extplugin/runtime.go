package extplugin

import (
	"context"
	"fmt"
	"os"

	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// LoadOne spawns and registers a single freshly-installed plugin binary at
// runtime (the marketplace install path).
func (m *Manager) LoadOne(ctx context.Context, reg *pluginregistry.Registry, path string) error {
	return m.load(ctx, reg, path)
}

// Update swaps a loaded plugin for the new binary at path: the new subprocess
// must present the same plugin name, the registry entry is replaced, and only
// then is the old subprocess stopped. Live sessions of the old version are
// dropped; the session registry reconnects lazily.
func (m *Manager) Update(ctx context.Context, reg *pluginregistry.Registry, name, path string) error {
	m.mu.Lock()
	var old *managed
	for _, mp := range m.managed {
		if mp.name == name {
			old = mp
			break
		}
	}
	m.mu.Unlock()
	if old == nil {
		return fmt.Errorf("plugin %q: %w", name, plugin.ErrNotFound)
	}

	client, dispensed, err := m.spawn(path)
	if err != nil {
		return err
	}
	ref := &clientRef{client: dispensed.Plugin, broker: dispensed.Broker}
	p, err := newPlugin(ctx, ref, m.audit)
	if err != nil {
		client.Kill()
		return err
	}
	if got := p.Manifest().Name; got != name {
		client.Kill()
		return fmt.Errorf("%w: binary presents plugin %q, expected %q", plugin.ErrInvalidInput, got, name)
	}
	if err := reg.Replace(p); err != nil {
		client.Kill()
		return err
	}

	mp := &managed{name: name, path: path, ref: ref, stop: make(chan struct{}), client: client}
	m.mu.Lock()
	for i, cur := range m.managed {
		if cur == old {
			m.managed[i] = mp
			break
		}
	}
	m.mu.Unlock()
	go m.supervise(mp)

	close(old.stop)
	old.mu.Lock()
	old.stopped = true
	oldClient := old.client
	old.mu.Unlock()
	oldClient.Kill()
	return nil
}

// Uninstall stops a managed external plugin, removes it from the runtime
// registry, and deletes the installed binary.
func (m *Manager) Uninstall(reg *pluginregistry.Registry, name string) error {
	m.mu.Lock()
	var old *managed
	var idx int
	for i, mp := range m.managed {
		if mp.name == name {
			old = mp
			idx = i
			break
		}
	}
	if old == nil {
		m.mu.Unlock()
		return fmt.Errorf("plugin %q: %w", name, plugin.ErrNotFound)
	}
	m.managed = append(m.managed[:idx], m.managed[idx+1:]...)
	m.mu.Unlock()

	if err := reg.Unregister(name); err != nil {
		m.mu.Lock()
		m.managed = append(m.managed, old)
		m.mu.Unlock()
		return err
	}

	close(old.stop)
	old.mu.Lock()
	old.stopped = true
	oldClient := old.client
	path := old.path
	old.mu.Unlock()
	oldClient.Kill()

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsManaged reports whether name is a loaded external plugin.
func (m *Manager) IsManaged(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mp := range m.managed {
		if mp.name == name {
			return true
		}
	}
	return false
}
