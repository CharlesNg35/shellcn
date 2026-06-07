package server

import (
	"context"
	"fmt"
	"time"

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

type storageBridge struct {
	inner        store.PluginStorageStore
	pluginName   string
	connectionID string
	ownerID      string
}

func (b storageBridge) Get(ctx context.Context, scope plugin.StorageScope, key string) (plugin.StorageItem, error) {
	if key == "" {
		return plugin.StorageItem{}, fmt.Errorf("%w: storage key is required", plugin.ErrInvalidInput)
	}
	f, err := b.filter(scope, key, "")
	if err != nil {
		return plugin.StorageItem{}, err
	}
	item, err := b.inner.Get(ctx, f)
	if err != nil {
		return plugin.StorageItem{}, err
	}
	return toPluginStorageItem(item), nil
}

func (b storageBridge) Put(ctx context.Context, item plugin.StorageItem) (plugin.StorageItem, error) {
	if item.Key == "" {
		return plugin.StorageItem{}, fmt.Errorf("%w: storage key is required", plugin.ErrInvalidInput)
	}
	scope, err := b.scope(item.Scope)
	if err != nil {
		return plugin.StorageItem{}, err
	}
	now := time.Now()
	row := toModelStorageItem(item)
	row.Namespace = scope.Namespace
	row.Plugin = scope.Plugin
	row.Protocol = scope.Protocol
	row.ConnectionID = scope.ConnectionID
	row.OwnerID = scope.OwnerID
	row.UserScoped = scope.UserScoped
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if err := b.inner.Put(ctx, &row); err != nil {
		return plugin.StorageItem{}, err
	}
	return toPluginStorageItem(row), nil
}

func (b storageBridge) Delete(ctx context.Context, scope plugin.StorageScope, key string) error {
	if key == "" {
		return fmt.Errorf("%w: storage key is required", plugin.ErrInvalidInput)
	}
	f, err := b.filter(scope, key, "")
	if err != nil {
		return err
	}
	return b.inner.Delete(ctx, f)
}

func (b storageBridge) List(ctx context.Context, scope plugin.StorageScope, prefix string) ([]plugin.StorageItem, error) {
	f, err := b.filter(scope, "", prefix)
	if err != nil {
		return nil, err
	}
	rows, err := b.inner.List(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.StorageItem, len(rows))
	for i, row := range rows {
		out[i] = toPluginStorageItem(row)
	}
	return out, nil
}

func (b storageBridge) filter(scope plugin.StorageScope, key, prefix string) (store.PluginStorageFilter, error) {
	normalized, err := b.scope(scope)
	if err != nil {
		return store.PluginStorageFilter{}, err
	}
	return store.PluginStorageFilter{
		Namespace:    normalized.Namespace,
		Plugin:       normalized.Plugin,
		Protocol:     normalized.Protocol,
		ConnectionID: normalized.ConnectionID,
		OwnerID:      normalized.OwnerID,
		UserScoped:   &normalized.UserScoped,
		Key:          key,
		Prefix:       prefix,
	}, nil
}

type resolvedStorageScope struct {
	Namespace    string
	Plugin       string
	Protocol     string
	ConnectionID string
	OwnerID      string
	UserScoped   bool
}

func (b storageBridge) scope(scope plugin.StorageScope) (resolvedStorageScope, error) {
	if scope.Namespace == "" {
		return resolvedStorageScope{}, fmt.Errorf("%w: storage namespace is required", plugin.ErrInvalidInput)
	}
	out := resolvedStorageScope{
		Namespace:  scope.Namespace,
		Plugin:     b.pluginName,
		Protocol:   b.pluginName,
		OwnerID:    b.ownerID,
		UserScoped: scope.UserScoped,
	}
	if !scope.UserScoped {
		out.ConnectionID = b.connectionID
	}
	return out, nil
}

func toModelStorageItem(item plugin.StorageItem) models.PluginStorageItem {
	return models.PluginStorageItem{
		Namespace:   item.Scope.Namespace,
		UserScoped:  item.Scope.UserScoped,
		ItemKey:     item.Key,
		Value:       append([]byte(nil), item.Value...),
		ContentType: item.ContentType,
		Metadata:    cloneMap(item.Metadata),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func toPluginStorageItem(item models.PluginStorageItem) plugin.StorageItem {
	return plugin.StorageItem{
		Scope: plugin.StorageScope{
			Namespace:  item.Namespace,
			UserScoped: item.UserScoped,
		},
		Key:         item.ItemKey,
		Value:       append([]byte(nil), item.Value...),
		ContentType: item.ContentType,
		Metadata:    cloneMap(item.Metadata),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
