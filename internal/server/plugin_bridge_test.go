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
	bridge := storageBridge{inner: st, pluginName: "ssh", connectionID: "c1", ownerID: "u1"}

	item, err := bridge.Put(context.Background(), plugin.StorageItem{
		Scope: plugin.StorageScope{Namespace: "private"},
		Key:   "k",
		Value: []byte("v"),
	})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if item.Scope.Namespace != "private" || item.Scope.UserScoped {
		t.Fatalf("private SDK scope leaked core fields: %+v", item.Scope)
	}
	if st.item.Plugin != "ssh" || st.item.Protocol != "ssh" || st.item.ConnectionID != "c1" || st.item.OwnerID != "u1" || st.item.UserScoped {
		t.Fatalf("private storage was not locked to current context: %+v", st.item)
	}
}

func TestStorageBridgeUserScopeIsPluginAndOwnerBound(t *testing.T) {
	st := &capturePluginStorage{}
	bridge := storageBridge{inner: st, pluginName: "ssh", connectionID: "c1", ownerID: "u1"}

	item, err := bridge.Put(context.Background(), plugin.StorageItem{
		Scope: plugin.StorageScope{Namespace: "snippets", UserScoped: true},
		Key:   "snippet-1",
		Value: []byte("whoami"),
	})
	if err != nil {
		t.Fatalf("put user-scoped: %v", err)
	}
	if item.Scope.Namespace != "snippets" || !item.Scope.UserScoped {
		t.Fatalf("user-scoped SDK scope not preserved: %+v", item.Scope)
	}
	if st.item.Plugin != "ssh" || st.item.Protocol != "ssh" || st.item.ConnectionID != "" || st.item.OwnerID != "u1" || !st.item.UserScoped {
		t.Fatalf("user-scoped storage was not locked to plugin and owner: %+v", st.item)
	}
}

type capturePluginStorage struct {
	item models.PluginStorageItem
}

func (s *capturePluginStorage) Get(_ context.Context, f store.PluginStorageFilter) (models.PluginStorageItem, error) {
	if s.item.Namespace != f.Namespace || s.item.Plugin != f.Plugin || s.item.ConnectionID != f.ConnectionID ||
		s.item.OwnerID != f.OwnerID || s.item.UserScoped != boolValue(f.UserScoped) || s.item.ItemKey != f.Key {
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

func boolValue(v *bool) bool {
	return v != nil && *v
}
