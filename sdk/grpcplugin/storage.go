package grpcplugin

import (
	"context"
	"time"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type hostStorage struct {
	host pluginv1.HostClient
}

func newHostStorage(host pluginv1.HostClient) plugin.Storage {
	if host == nil {
		return nil
	}
	return hostStorage{host: host}
}

func (s hostStorage) Get(ctx context.Context, scope plugin.StorageScope, key string) (plugin.StorageItem, error) {
	item, err := s.host.StorageGet(ctx, &pluginv1.StorageGetRequest{Scope: wireStorageScope(scope), Key: key})
	if err != nil {
		return plugin.StorageItem{}, ErrorFromStatus(err)
	}
	return pluginStorageItem(item), nil
}

func (s hostStorage) Put(ctx context.Context, collection string, item plugin.StorageItem) (plugin.StorageItem, error) {
	stored, err := s.host.StoragePut(ctx, &pluginv1.StoragePutRequest{Collection: collection, Item: wireStorageItem(item)})
	if err != nil {
		return plugin.StorageItem{}, ErrorFromStatus(err)
	}
	return pluginStorageItem(stored), nil
}

func (s hostStorage) Delete(ctx context.Context, scope plugin.StorageScope, key string) error {
	_, err := s.host.StorageDelete(ctx, &pluginv1.StorageDeleteRequest{Scope: wireStorageScope(scope), Key: key})
	return ErrorFromStatus(err)
}

func (s hostStorage) List(ctx context.Context, scope plugin.StorageScope, filter *plugin.StorageListFilter) ([]plugin.StorageItem, error) {
	resp, err := s.host.StorageList(ctx, &pluginv1.StorageListRequest{Scope: wireStorageScope(scope), Filter: wireStorageListFilter(filter)})
	if err != nil {
		return nil, ErrorFromStatus(err)
	}
	out := make([]plugin.StorageItem, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		out[i] = pluginStorageItem(item)
	}
	return out, nil
}

func wireStorageListFilter(filter *plugin.StorageListFilter) *pluginv1.StorageListFilter {
	if filter == nil {
		return nil
	}
	return &pluginv1.StorageListFilter{
		Keys:                  append([]string(nil), filter.Keys...),
		KeyPrefix:             filter.KeyPrefix,
		ContentType:           filter.ContentType,
		CreatedAfterUnixNano:  timeUnixNano(filter.CreatedAfter),
		CreatedBeforeUnixNano: timeUnixNano(filter.CreatedBefore),
		UpdatedAfterUnixNano:  timeUnixNano(filter.UpdatedAfter),
		UpdatedBeforeUnixNano: timeUnixNano(filter.UpdatedBefore),
		Limit:                 int32(filter.Limit),
		Offset:                int32(filter.Offset),
	}
}

func wireStorageScope(scope plugin.StorageScope) *pluginv1.StorageScope {
	return &pluginv1.StorageScope{
		Collection: scope.Collection,
		Level:      string(normalizeStorageScopeLevel(scope.Level)),
	}
}

func pluginStorageListFilter(filter *pluginv1.StorageListFilter) *plugin.StorageListFilter {
	if filter == nil {
		return nil
	}
	return &plugin.StorageListFilter{
		Keys:          append([]string(nil), filter.GetKeys()...),
		KeyPrefix:     filter.GetKeyPrefix(),
		ContentType:   filter.GetContentType(),
		CreatedAfter:  unixNanoTime(filter.GetCreatedAfterUnixNano()),
		CreatedBefore: unixNanoTime(filter.GetCreatedBeforeUnixNano()),
		UpdatedAfter:  unixNanoTime(filter.GetUpdatedAfterUnixNano()),
		UpdatedBefore: unixNanoTime(filter.GetUpdatedBeforeUnixNano()),
		Limit:         int(filter.GetLimit()),
		Offset:        int(filter.GetOffset()),
	}
}

func normalizeStorageScopeLevel(level plugin.StorageScopeLevel) plugin.StorageScopeLevel {
	if level == "" {
		return plugin.StorageScopeConnection
	}
	return level
}

func wireStorageItem(item plugin.StorageItem) *pluginv1.StorageItem {
	return &pluginv1.StorageItem{
		Key:               item.Key,
		Value:             append([]byte(nil), item.Value...),
		ContentType:       item.ContentType,
		Metadata:          cloneStringMap(item.Metadata),
		CreatedAtUnixNano: item.CreatedAt.UnixNano(),
		UpdatedAtUnixNano: item.UpdatedAt.UnixNano(),
	}
}

func pluginStorageItem(item *pluginv1.StorageItem) plugin.StorageItem {
	if item == nil {
		return plugin.StorageItem{}
	}
	return plugin.StorageItem{
		Key:         item.GetKey(),
		Value:       append([]byte(nil), item.GetValue()...),
		ContentType: item.GetContentType(),
		Metadata:    cloneStringMap(item.GetMetadata()),
		CreatedAt:   unixNanoTime(item.GetCreatedAtUnixNano()),
		UpdatedAt:   unixNanoTime(item.GetUpdatedAtUnixNano()),
	}
}

func unixNanoTime(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, v)
}

func timeUnixNano(v time.Time) int64 {
	if v.IsZero() {
		return 0
	}
	return v.UnixNano()
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
