package server

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestStorageBridgeLocksPrivateScopeToCurrentContext(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}

	item, err := bridge.Put(context.Background(), "private", plugin.StorageItem{Key: "k", Value: []byte("v")})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if item.Key != "k" || string(item.Value) != "v" {
		t.Fatalf("unexpected stored item: %+v", item)
	}
	if st.item.Collection != "private" || st.item.Plugin != "ssh" || st.item.ConnectionID != "c1" || st.item.OwnerID != "u1" {
		t.Fatalf("private storage was not locked to current context: %+v", st.item)
	}
}

func TestStorageBridgeOwnsWriteTimestamps(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}
	old := time.Date(2001, 2, 3, 4, 5, 6, 7, time.UTC)

	item, err := bridge.Put(context.Background(), "snippets", plugin.StorageItem{
		Key:       "k",
		CreatedAt: old,
		UpdatedAt: old,
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if !st.item.CreatedAt.After(old) || !st.item.UpdatedAt.After(old) {
		t.Fatalf("storage bridge should own timestamps, got created=%s updated=%s", st.item.CreatedAt, st.item.UpdatedAt)
	}
	if !item.CreatedAt.Equal(st.item.CreatedAt) || !item.UpdatedAt.Equal(st.item.UpdatedAt) {
		t.Fatalf("returned timestamps should match stored row: item=%+v stored=%+v", item, st.item)
	}
}

func TestStorageBridgeUserScopeFiltersByPluginAndOwner(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}
	st.item = models.PluginStorageItem{
		Collection: "snippets", Plugin: "ssh", ConnectionID: "other-connection", OwnerID: "u1",
		ItemKey: "snippet-1", Value: []byte("whoami"),
	}

	item, err := bridge.Get(context.Background(), plugin.UserStorage("snippets"), "snippet-1")
	if err != nil {
		t.Fatalf("get user-scoped: %v", err)
	}
	if string(item.Value) != "whoami" {
		t.Fatalf("unexpected user-scoped item: %+v", item)
	}
	if st.lastFilter.ConnectionID != "" {
		t.Fatalf("user storage should not filter by connection: %+v", st.lastFilter)
	}
}

func TestStorageBridgeListAppliesCallerFilterInsideResolvedScope(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}

	_, err := bridge.List(context.Background(), plugin.ConnectionStorage("snippets"), &plugin.StorageListFilter{
		Keys:          []string{"prod/restart", "prod/status"},
		KeyPrefix:     "prod/",
		ContentType:   "application/json",
		CreatedAfter:  time.Unix(10, 0),
		CreatedBefore: time.Unix(20, 0),
		UpdatedAfter:  time.Unix(30, 0),
		UpdatedBefore: time.Unix(40, 0),
		Limit:         25,
		Offset:        50,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if st.lastFilter.Collection != "snippets" ||
		st.lastFilter.Plugin != "ssh" ||
		st.lastFilter.ConnectionID != "c1" ||
		st.lastFilter.OwnerID != "u1" ||
		!slices.Equal(st.lastFilter.Keys, []string{"prod/restart", "prod/status"}) ||
		st.lastFilter.KeyPrefix != "prod/" ||
		st.lastFilter.ContentType != "application/json" ||
		!st.lastFilter.CreatedAfter.Equal(time.Unix(10, 0)) ||
		!st.lastFilter.CreatedBefore.Equal(time.Unix(20, 0)) ||
		!st.lastFilter.UpdatedAfter.Equal(time.Unix(30, 0)) ||
		!st.lastFilter.UpdatedBefore.Equal(time.Unix(40, 0)) ||
		st.lastFilter.Limit != 25 ||
		st.lastFilter.Offset != 50 {
		t.Fatalf("unexpected list filter: %+v", st.lastFilter)
	}
}

func TestStorageBridgeMapsStoreErrors(t *testing.T) {
	st := &capturePluginStorage{getErr: models.ErrConflict}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}

	if _, err := bridge.Get(context.Background(), plugin.UserStorage("snippets"), "snippet-1"); !errors.Is(err, plugin.ErrConflict) {
		t.Fatalf("get conflict: want plugin.ErrConflict, got %v", err)
	}

	st.getErr = store.ErrNotFound
	if _, err := bridge.Get(context.Background(), plugin.UserStorage("snippets"), "snippet-1"); !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("get missing: want plugin.ErrNotFound, got %v", err)
	}
}

func TestStorageBridgePutRequiresResolvedContext(t *testing.T) {
	for _, tc := range []struct {
		name       string
		bridge     storageBridge
		collection string
		item       plugin.StorageItem
	}{
		{
			name:       "collection",
			bridge:     storageBridge{pluginID: "ssh", connectionID: "c1", ownerID: "u1"},
			collection: "",
			item:       plugin.StorageItem{Key: "k"},
		},
		{
			name:       "plugin",
			bridge:     storageBridge{connectionID: "c1", ownerID: "u1"},
			collection: "snippets",
			item:       plugin.StorageItem{Key: "k"},
		},
		{
			name:       "connection",
			bridge:     storageBridge{pluginID: "ssh", ownerID: "u1"},
			collection: "snippets",
			item:       plugin.StorageItem{Key: "k"},
		},
		{
			name:       "owner",
			bridge:     storageBridge{pluginID: "ssh", connectionID: "c1"},
			collection: "snippets",
			item:       plugin.StorageItem{Key: "k"},
		},
		{
			name:       "key",
			bridge:     storageBridge{pluginID: "ssh", connectionID: "c1", ownerID: "u1"},
			collection: "snippets",
			item:       plugin.StorageItem{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &capturePluginStorage{}
			tc.bridge.inner = st
			if _, err := tc.bridge.Put(context.Background(), tc.collection, tc.item); !errors.Is(err, plugin.ErrInvalidInput) {
				t.Fatalf("put invalid %s: want ErrInvalidInput, got %v", tc.name, err)
			}
			if st.putCalls != 0 {
				t.Fatalf("storage put should not be called for invalid %s", tc.name)
			}
		})
	}
}

type capturePluginStorage struct {
	item       models.PluginStorageItem
	lastFilter store.PluginStorageFilter
	getErr     error
	putCalls   int
}

func (s *capturePluginStorage) Get(_ context.Context, f store.PluginStorageFilter) (models.PluginStorageItem, error) {
	s.lastFilter = f
	if s.getErr != nil {
		return models.PluginStorageItem{}, s.getErr
	}
	if s.item.Collection != f.Collection || s.item.Plugin != f.Plugin || s.item.OwnerID != f.OwnerID || s.item.ItemKey != f.Key {
		return models.PluginStorageItem{}, store.ErrNotFound
	}
	if f.ConnectionID != "" && s.item.ConnectionID != f.ConnectionID {
		return models.PluginStorageItem{}, store.ErrNotFound
	}
	return s.item, nil
}

func (s *capturePluginStorage) Put(_ context.Context, item *models.PluginStorageItem) error {
	s.putCalls++
	s.item = *item
	s.item.Value = append([]byte(nil), item.Value...)
	return nil
}

func (s *capturePluginStorage) Delete(context.Context, store.PluginStorageFilter) error {
	return nil
}

func (s *capturePluginStorage) List(_ context.Context, f store.PluginStorageFilter) ([]models.PluginStorageItem, error) {
	s.lastFilter = f
	return []models.PluginStorageItem{s.item}, nil
}
