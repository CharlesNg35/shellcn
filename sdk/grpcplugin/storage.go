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

func (s hostStorage) Put(ctx context.Context, item plugin.StorageItem) (plugin.StorageItem, error) {
	stored, err := s.host.StoragePut(ctx, wireStorageItem(item))
	if err != nil {
		return plugin.StorageItem{}, ErrorFromStatus(err)
	}
	return pluginStorageItem(stored), nil
}

func (s hostStorage) Delete(ctx context.Context, scope plugin.StorageScope, key string) error {
	_, err := s.host.StorageDelete(ctx, &pluginv1.StorageDeleteRequest{Scope: wireStorageScope(scope), Key: key})
	return ErrorFromStatus(err)
}

func (s hostStorage) List(ctx context.Context, scope plugin.StorageScope, prefix string) ([]plugin.StorageItem, error) {
	resp, err := s.host.StorageList(ctx, &pluginv1.StorageListRequest{Scope: wireStorageScope(scope), Prefix: prefix})
	if err != nil {
		return nil, ErrorFromStatus(err)
	}
	out := make([]plugin.StorageItem, len(resp.GetItems()))
	for i, item := range resp.GetItems() {
		out[i] = pluginStorageItem(item)
	}
	return out, nil
}

func wireStorageScope(scope plugin.StorageScope) *pluginv1.StorageScope {
	return &pluginv1.StorageScope{
		Namespace:  scope.Namespace,
		UserScoped: scope.UserScoped,
	}
}

func pluginStorageScope(scope *pluginv1.StorageScope) plugin.StorageScope {
	if scope == nil {
		return plugin.StorageScope{}
	}
	return plugin.StorageScope{
		Namespace:  scope.GetNamespace(),
		UserScoped: scope.GetUserScoped(),
	}
}

func wireStorageItem(item plugin.StorageItem) *pluginv1.StorageItem {
	return &pluginv1.StorageItem{
		Scope:             wireStorageScope(item.Scope),
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
		Scope:       pluginStorageScope(item.GetScope()),
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
