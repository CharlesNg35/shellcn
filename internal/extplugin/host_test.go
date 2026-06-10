package extplugin

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/gen/pluginv1"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestHostServerStorageListPassesFilter(t *testing.T) {
	storage := &captureHostStorage{}
	host := newHostServer(nil, storage, nil, nil)

	_, err := host.StorageList(context.Background(), &pluginv1.StorageListRequest{
		Scope: &pluginv1.StorageScope{Collection: "snippets", Level: string(plugin.StorageScopeUser)},
		Filter: &pluginv1.StorageListFilter{
			Keys:                  []string{"prod/restart", "prod/status"},
			KeyPrefix:             "prod/",
			ContentType:           "application/json",
			CreatedAfterUnixNano:  time.Unix(10, 1).UnixNano(),
			CreatedBeforeUnixNano: time.Unix(20, 2).UnixNano(),
			UpdatedAfterUnixNano:  time.Unix(30, 3).UnixNano(),
			UpdatedBeforeUnixNano: time.Unix(40, 4).UnixNano(),
			Limit:                 25,
			Offset:                50,
		},
	})
	if err != nil {
		t.Fatalf("storage list: %v", err)
	}
	if storage.scope.Collection != "snippets" ||
		storage.scope.Level != plugin.StorageScopeUser ||
		storage.filter == nil ||
		!slices.Equal(storage.filter.Keys, []string{"prod/restart", "prod/status"}) ||
		storage.filter.KeyPrefix != "prod/" ||
		storage.filter.ContentType != "application/json" ||
		!storage.filter.CreatedAfter.Equal(time.Unix(10, 1)) ||
		!storage.filter.CreatedBefore.Equal(time.Unix(20, 2)) ||
		!storage.filter.UpdatedAfter.Equal(time.Unix(30, 3)) ||
		!storage.filter.UpdatedBefore.Equal(time.Unix(40, 4)) ||
		storage.filter.Limit != 25 ||
		storage.filter.Offset != 50 {
		t.Fatalf("unexpected storage list call: scope=%+v filter=%+v", storage.scope, storage.filter)
	}
}

type captureHostStorage struct {
	scope  plugin.StorageScope
	filter *plugin.StorageListFilter
}

func (s *captureHostStorage) Get(context.Context, plugin.StorageScope, string) (plugin.StorageItem, error) {
	return plugin.StorageItem{}, plugin.ErrNotFound
}

func (s *captureHostStorage) Put(context.Context, string, plugin.StorageItem) (plugin.StorageItem, error) {
	return plugin.StorageItem{}, nil
}

func (s *captureHostStorage) Delete(context.Context, plugin.StorageScope, string) error {
	return nil
}

func (s *captureHostStorage) List(_ context.Context, scope plugin.StorageScope, filter *plugin.StorageListFilter) ([]plugin.StorageItem, error) {
	s.scope = scope
	s.filter = filter
	return []plugin.StorageItem{{Key: "prod/restart"}}, nil
}
