package sshsftp

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestSnippetStoreUsesGenericPluginStorage(t *testing.T) {
	storage := newTestPluginStorage()
	snippets := newSnippetStore(storage)
	ctx := context.Background()

	first := &storedSnippet{ID: "b", Name: "Beta", Body: "echo beta"}
	second := &storedSnippet{ID: "a", Name: "alpha", Body: "echo alpha"}
	if err := snippets.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := snippets.Create(ctx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}

	got, err := snippets.Get(ctx, "a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "alpha" || got.Body != "echo alpha" {
		t.Fatalf("unexpected snippet: %+v", got)
	}

	rows, err := snippets.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	names := []string{rows[0].Name, rows[1].Name}
	if !slices.Equal(names, []string{"alpha", "Beta"}) {
		t.Fatalf("snippets should sort case-insensitively by name: %+v", rows)
	}

	if err := snippets.Delete(ctx, "a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := snippets.Get(ctx, "a"); !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

type testStorageKey struct {
	scope plugin.StorageScope
	key   string
}

type testPluginStorage struct {
	items map[testStorageKey]plugin.StorageItem
}

func newTestPluginStorage() *testPluginStorage {
	return &testPluginStorage{items: map[testStorageKey]plugin.StorageItem{}}
}

func (s *testPluginStorage) Get(_ context.Context, scope plugin.StorageScope, key string) (plugin.StorageItem, error) {
	item, ok := s.items[testStorageKey{scope: scope, key: key}]
	if !ok {
		return plugin.StorageItem{}, plugin.ErrNotFound
	}
	return cloneStorageItem(item), nil
}

func (s *testPluginStorage) Put(_ context.Context, item plugin.StorageItem) (plugin.StorageItem, error) {
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	s.items[testStorageKey{scope: item.Scope, key: item.Key}] = cloneStorageItem(item)
	return cloneStorageItem(item), nil
}

func (s *testPluginStorage) Delete(_ context.Context, scope plugin.StorageScope, key string) error {
	k := testStorageKey{scope: scope, key: key}
	if _, ok := s.items[k]; !ok {
		return plugin.ErrNotFound
	}
	delete(s.items, k)
	return nil
}

func (s *testPluginStorage) List(_ context.Context, scope plugin.StorageScope, _ string) ([]plugin.StorageItem, error) {
	var out []plugin.StorageItem
	for k, item := range s.items {
		if k.scope == scope {
			out = append(out, cloneStorageItem(item))
		}
	}
	return out, nil
}

func cloneStorageItem(item plugin.StorageItem) plugin.StorageItem {
	item.Value = append([]byte(nil), item.Value...)
	if len(item.Metadata) > 0 {
		metadata := make(map[string]string, len(item.Metadata))
		for k, v := range item.Metadata {
			metadata[k] = v
		}
		item.Metadata = metadata
	}
	return item
}
