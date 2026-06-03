package extplugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/charlesng35/shellcn/sdk/grpcplugin"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	pollInterval   = 500 * time.Millisecond
	minRespawnWait = 200 * time.Millisecond
	maxRespawnWait = 30 * time.Second
)

// Manager discovers, spawns, and supervises out-of-process plugin binaries. A
// crashed plugin is respawned with bounded backoff; its registered manifest and
// routes are unchanged — only the live gRPC client is swapped underneath.
type Manager struct {
	dir    string
	logger hclog.Logger
	audit  AuditFunc

	mu      sync.Mutex
	managed []*managed
}

// Option configures a Manager.
type Option func(*Manager)

// WithAudit records stream-internal operations plugins report via Host.Audit.
func WithAudit(audit AuditFunc) Option {
	return func(m *Manager) { m.audit = audit }
}

type managed struct {
	name string
	path string
	ref  *clientRef
	stop chan struct{}

	mu      sync.Mutex
	client  *goplugin.Client
	stopped bool
}

// Loaded describes one currently-loaded external plugin for the admin surface.
type Loaded struct {
	Name    string
	Path    string
	Healthy bool
}

// Loaded returns a snapshot of every loaded external plugin and whether its
// subprocess is currently running.
func (m *Manager) Loaded() []Loaded {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Loaded, 0, len(m.managed))
	for _, mp := range m.managed {
		mp.mu.Lock()
		healthy := mp.client != nil && !mp.client.Exited()
		mp.mu.Unlock()
		out = append(out, Loaded{Name: mp.name, Path: mp.path, Healthy: healthy})
	}
	return out
}

func NewManager(dir string, opts ...Option) *Manager {
	m := &Manager{dir: dir, logger: hclog.NewNullLogger()}
	for _, o := range opts {
		o(m)
	}
	return m
}

// LoadAll spawns and registers every plugin binary in the directory. A binary
// that fails to load is skipped so one bad plugin cannot block the rest; the
// joined load errors are returned. A missing directory is not an error.
func (m *Manager) LoadAll(ctx context.Context, reg *plugin.Registry) error {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Mode()&0o111 == 0 {
			continue
		}
		path := filepath.Join(m.dir, e.Name())
		if err := m.load(ctx, reg, path); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) load(ctx context.Context, reg *plugin.Registry, path string) error {
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
	if err := reg.Register(p); err != nil {
		client.Kill()
		return err
	}

	mp := &managed{name: p.Manifest().Name, path: path, ref: ref, stop: make(chan struct{}), client: client}
	m.mu.Lock()
	m.managed = append(m.managed, mp)
	m.mu.Unlock()
	go m.supervise(mp)
	return nil
}

func (m *Manager) spawn(path string) (*goplugin.Client, *grpcplugin.Client, error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  grpcplugin.Handshake,
		Plugins:          grpcplugin.Plugins(nil),
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		AutoMTLS:         true,
		Logger:           m.logger,
	})
	rpc, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, err
	}
	raw, err := rpc.Dispense(grpcplugin.PluginName)
	if err != nil {
		client.Kill()
		return nil, nil, err
	}
	dispensed, ok := raw.(*grpcplugin.Client)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("unexpected plugin type %T", raw)
	}
	return client, dispensed, nil
}

// supervise watches a plugin for an unexpected exit and respawns it with bounded
// backoff, swapping the live client so the registered plugin keeps working.
func (m *Manager) supervise(mp *managed) {
	for {
		select {
		case <-mp.stop:
			return
		case <-time.After(pollInterval):
		}

		mp.mu.Lock()
		client, stopped := mp.client, mp.stopped
		mp.mu.Unlock()
		if stopped {
			return
		}
		if !client.Exited() {
			continue
		}
		if !m.respawn(mp) {
			return
		}
	}
}

func (m *Manager) respawn(mp *managed) bool {
	wait := minRespawnWait
	for {
		select {
		case <-mp.stop:
			return false
		case <-time.After(wait):
		}

		mp.mu.Lock()
		stopped := mp.stopped
		mp.mu.Unlock()
		if stopped {
			return false
		}

		client, dispensed, err := m.spawn(mp.path)
		if err != nil {
			m.logger.Warn("respawn failed", "path", mp.path, "err", err)
			wait = min(wait*2, maxRespawnWait)
			continue
		}
		mp.ref.set(dispensed.Plugin, dispensed.Broker)
		mp.mu.Lock()
		mp.client = client
		mp.mu.Unlock()
		return true
	}
}

// Close terminates every spawned subprocess and stops its supervisor.
func (m *Manager) Close() {
	m.mu.Lock()
	plugins := m.managed
	m.managed = nil
	m.mu.Unlock()
	for _, mp := range plugins {
		close(mp.stop)
		mp.mu.Lock()
		mp.stopped = true
		client := mp.client
		mp.mu.Unlock()
		client.Kill()
	}
}
