package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	pluginID     string
	connectionID string
	ownerID      string
}

func (b storageBridge) Get(ctx context.Context, scope plugin.StorageScope, key string) (plugin.StorageItem, error) {
	if key == "" {
		return plugin.StorageItem{}, fmt.Errorf("%w: storage key is required", plugin.ErrInvalidInput)
	}
	f, err := b.filter(scope, key)
	if err != nil {
		return plugin.StorageItem{}, err
	}
	item, err := b.inner.Get(ctx, f)
	if err != nil {
		return plugin.StorageItem{}, pluginStorageError(err)
	}
	return toPluginStorageItem(item), nil
}

func (b storageBridge) Put(ctx context.Context, collection string, item plugin.StorageItem) (plugin.StorageItem, error) {
	now := time.Now()
	row := toModelStorageItem(item)
	row.Collection = collection
	row.Plugin = b.pluginID
	row.ConnectionID = b.connectionID
	row.OwnerID = b.ownerID
	row.CreatedAt = now
	row.UpdatedAt = now
	if err := row.Validate(); err != nil {
		return plugin.StorageItem{}, pluginStorageError(err)
	}
	if err := b.inner.Put(ctx, &row); err != nil {
		return plugin.StorageItem{}, pluginStorageError(err)
	}
	return toPluginStorageItem(row), nil
}

func (b storageBridge) Delete(ctx context.Context, scope plugin.StorageScope, key string) error {
	if key == "" {
		return fmt.Errorf("%w: storage key is required", plugin.ErrInvalidInput)
	}
	f, err := b.filter(scope, key)
	if err != nil {
		return err
	}
	return pluginStorageError(b.inner.Delete(ctx, f))
}

func (b storageBridge) List(ctx context.Context, scope plugin.StorageScope, filter *plugin.StorageListFilter) ([]plugin.StorageItem, error) {
	f, err := b.filter(scope, "")
	if err != nil {
		return nil, err
	}
	applyStorageListFilter(&f, filter)
	rows, err := b.inner.List(ctx, f)
	if err != nil {
		return nil, pluginStorageError(err)
	}
	out := make([]plugin.StorageItem, len(rows))
	for i, row := range rows {
		out[i] = toPluginStorageItem(row)
	}
	return out, nil
}

func applyStorageListFilter(f *store.PluginStorageFilter, filter *plugin.StorageListFilter) {
	if filter == nil {
		return
	}
	f.Keys = append([]string(nil), filter.Keys...)
	f.KeyPrefix = filter.KeyPrefix
	f.ContentType = filter.ContentType
	f.CreatedAfter = filter.CreatedAfter
	f.CreatedBefore = filter.CreatedBefore
	f.UpdatedAfter = filter.UpdatedAfter
	f.UpdatedBefore = filter.UpdatedBefore
	f.Limit = filter.Limit
	f.Offset = filter.Offset
}

func (b storageBridge) filter(scope plugin.StorageScope, key string) (store.PluginStorageFilter, error) {
	normalized, err := b.scope(scope)
	if err != nil {
		return store.PluginStorageFilter{}, err
	}
	return store.PluginStorageFilter{
		Collection:   normalized.Collection,
		Plugin:       normalized.Plugin,
		ConnectionID: normalized.ConnectionID,
		OwnerID:      normalized.OwnerID,
		Key:          key,
	}, nil
}

type resolvedStorageScope struct {
	Collection   string
	Plugin       string
	ConnectionID string
	OwnerID      string
}

func (b storageBridge) scope(scope plugin.StorageScope) (resolvedStorageScope, error) {
	if scope.Collection == "" {
		return resolvedStorageScope{}, fmt.Errorf("%w: storage collection is required", plugin.ErrInvalidInput)
	}
	if b.pluginID == "" {
		return resolvedStorageScope{}, fmt.Errorf("%w: storage plugin scope is unavailable", plugin.ErrInvalidInput)
	}
	if b.ownerID == "" {
		return resolvedStorageScope{}, fmt.Errorf("%w: storage owner scope is unavailable", plugin.ErrInvalidInput)
	}
	out := resolvedStorageScope{
		Collection: scope.Collection,
		Plugin:     b.pluginID,
		OwnerID:    b.ownerID,
	}
	switch normalizeStorageScopeLevel(scope.Level) {
	case plugin.StorageScopeConnection:
		out.ConnectionID = b.connectionID
	case plugin.StorageScopeUser:
	default:
		return resolvedStorageScope{}, fmt.Errorf("%w: storage scope level is required", plugin.ErrInvalidInput)
	}
	if normalizeStorageScopeLevel(scope.Level) == plugin.StorageScopeConnection && out.ConnectionID == "" {
		return resolvedStorageScope{}, fmt.Errorf("%w: storage connection scope is unavailable", plugin.ErrInvalidInput)
	}
	return out, nil
}

func normalizeStorageScopeLevel(level plugin.StorageScopeLevel) plugin.StorageScopeLevel {
	if level == "" {
		return plugin.StorageScopeConnection
	}
	return level
}

func pluginStorageError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return plugin.ErrNotFound
	case errors.Is(err, models.ErrInvalidInput):
		return fmt.Errorf("%w: %s", plugin.ErrInvalidInput, storageErrorDetail(err, models.ErrInvalidInput))
	case errors.Is(err, models.ErrConflict):
		return plugin.ErrConflict
	default:
		return err
	}
}

func storageErrorDetail(err, sentinel error) string {
	return strings.TrimPrefix(err.Error(), sentinel.Error()+": ")
}

func toModelStorageItem(item plugin.StorageItem) models.PluginStorageItem {
	return models.PluginStorageItem{
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
