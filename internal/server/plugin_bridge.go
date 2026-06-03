package server

import (
	"context"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// toPluginUser maps the stored user to the lean identity handed to plugin
// handlers (authorization is already enforced before the handler runs).
func toPluginUser(u models.User) plugin.User {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	return plugin.User{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Roles: roles}
}

// snippetBridge adapts the store's snippet repository to the lean
// plugin.SnippetStore, mapping between the GORM model and the contract type.
type snippetBridge struct{ inner store.SnippetStore }

func toModelSnippet(s plugin.Snippet) models.Snippet {
	return models.Snippet{
		ID: s.ID, OwnerID: s.OwnerID, Protocol: s.Protocol,
		Name: s.Name, Body: s.Body, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func toPluginSnippet(s models.Snippet) plugin.Snippet {
	return plugin.Snippet{
		ID: s.ID, OwnerID: s.OwnerID, Protocol: s.Protocol,
		Name: s.Name, Body: s.Body, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func (b snippetBridge) Create(ctx context.Context, s *plugin.Snippet) error {
	m := toModelSnippet(*s)
	if err := b.inner.Create(ctx, &m); err != nil {
		return err
	}
	*s = toPluginSnippet(m)
	return nil
}

func (b snippetBridge) Get(ctx context.Context, id string) (plugin.Snippet, error) {
	m, err := b.inner.Get(ctx, id)
	return toPluginSnippet(m), err
}

func (b snippetBridge) ListByOwner(ctx context.Context, ownerID, protocol string) ([]plugin.Snippet, error) {
	rows, err := b.inner.ListByOwner(ctx, ownerID, protocol)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.Snippet, len(rows))
	for i, m := range rows {
		out[i] = toPluginSnippet(m)
	}
	return out, nil
}

func (b snippetBridge) Update(ctx context.Context, s *plugin.Snippet) error {
	m := toModelSnippet(*s)
	if err := b.inner.Update(ctx, &m); err != nil {
		return err
	}
	*s = toPluginSnippet(m)
	return nil
}

func (b snippetBridge) Delete(ctx context.Context, id string) error {
	return b.inner.Delete(ctx, id)
}
