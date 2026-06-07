package server

import (
	"context"
	"testing"

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
	if st.item.Namespace != "private" || st.item.Plugin != "ssh" || st.item.ConnectionID != "c1" || st.item.OwnerID != "u1" {
		t.Fatalf("private storage was not locked to current context: %+v", st.item)
	}
}

func TestStorageBridgeUserScopeFiltersByPluginAndOwner(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginID: "ssh", connectionID: "c1", ownerID: "u1"}
	st.item = models.PluginStorageItem{
		Namespace: "snippets", Plugin: "ssh", ConnectionID: "other-connection", OwnerID: "u1",
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

type capturePluginStorage struct {
	item       models.PluginStorageItem
	lastFilter store.PluginStorageFilter
}

func (s *capturePluginStorage) Get(_ context.Context, f store.PluginStorageFilter) (models.PluginStorageItem, error) {
	s.lastFilter = f
	if s.item.Namespace != f.Namespace || s.item.Plugin != f.Plugin || s.item.OwnerID != f.OwnerID || s.item.ItemKey != f.Key {
		return models.PluginStorageItem{}, store.ErrNotFound
	}
	if f.ConnectionID != "" && s.item.ConnectionID != f.ConnectionID {
		return models.PluginStorageItem{}, store.ErrNotFound
	}
	return s.item, nil
}

func (s *capturePluginStorage) Put(_ context.Context, item *models.PluginStorageItem) error {
	s.item = *item
	s.item.Value = append([]byte(nil), item.Value...)
	return nil
}

func (s *capturePluginStorage) Delete(context.Context, store.PluginStorageFilter) error {
	return nil
}

func (s *capturePluginStorage) List(context.Context, store.PluginStorageFilter) ([]models.PluginStorageItem, error) {
	return []models.PluginStorageItem{s.item}, nil
}
